// Ported from internal/plugin of egoist/mygo-music (AGPL-3.0, as this
// repository is), with its engine, moejs (Apache-2.0).

package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/MoeForge/moejs"
	"github.com/MoeForge/moejs/eventloop"
	"github.com/MoeForge/moejs/webapi"

	"netease-music-mygo/internal/music"
)

const (
	callTimeout = 30 * time.Second
	// Signing in waits for the listener, in their browser.
	signInTimeout = 5 * time.Minute
	// A runtime that doesn't take on a task in this long is stuck in the
	// plugin's code, rather than waiting on something.
	stuckAfter = time.Second
	// memoryLimit is what a plugin may keep. Passing it restarts the plugin.
	memoryLimit = 256 << 20
)

// Load reads and compiles the plugin in dir, keeping its data in dataDir,
// without running it. It starts on first use, and again on the next use after
// it got stuck.
func Load(dir, dataDir string) (*Plugin, error) {
	p, err := newPlugin(dir, dataDir)
	if err != nil {
		return nil, err
	}
	source, err := os.ReadFile(filepath.Join(dir, p.manifest.Main))
	if err != nil {
		return nil, err
	}
	compile := moejs.Compile
	if strings.HasSuffix(p.manifest.Main, ".ts") {
		compile = moejs.CompileTS
	}
	mod, err := compile(p.manifest.Main, string(source))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", dir, err)
	}
	p.mod = mod
	for _, name := range mod.Exports() {
		p.exports[name] = true
	}
	return p, nil
}

// Discover loads every plugin in the directories of dir. One that doesn't load
// is logged and left out: it shouldn't take the others with it.
func Discover(dir, dataDir string) []*Plugin {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var plugins []*Plugin
	for _, e := range entries {
		// A dot is what's on its way in, or anything else that's no plugin.
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		p, err := Load(filepath.Join(dir, e.Name()), dataDir)
		if err != nil {
			log.Printf("plugin: %v", err)
			continue
		}
		plugins = append(plugins, p)
	}
	return plugins
}

// Grant lets the plugin start programs, or stops it from doing so, keeping the
// choice. The plugin starts over for it: what it can reach depends on it, and
// taking it back ends the programs it started.
func (p *Plugin) Grant(on bool) error {
	err := p.keepGranted(on)
	p.Close()
	return err
}

// Close stops the plugin, and the programs it started.
func (p *Plugin) Close() {
	p.CancelSignIn()
	p.starting.Lock()
	defer p.starting.Unlock()
	p.mu.Lock()
	v := p.vm
	p.vm = nil
	p.mu.Unlock()
	if v != nil {
		v.close()
	}
}

// vm is a running plugin: only its loop's goroutine touches the runtime.
type vm struct {
	rt     *moejs.Runtime
	loop   *eventloop.Loop
	api    *webapi.API
	helper moejs.Value // calls an export and reports how it went

	inFlight int // on the loop: calls not answered yet

	mu       sync.Mutex // guards what follows
	programs map[*program]bool
	closed   bool
}

func (v *vm) close() {
	v.mu.Lock()
	v.closed = true
	v.mu.Unlock()
	v.killPrograms()
	v.api.Close()
	v.loop.Terminate()
}

// helperBody calls fn, whether it's sync or async, and says how it went
// through ok or bad. An error's code is read here, as Go only sees its message
// through the exception of what throws.
const helperBody = `
const fail = (e) => {
  const isObject = e !== null && typeof e === "object";
  bad(String((isObject && e.code) || ""), isObject ? String(e.message || "") : String(e));
};
try {
  Promise.resolve(fn(arg)).then((r) => ok(r === undefined ? null : r), fail);
} catch (e) {
  fail(e);
}`

var helperScript = func() *moejs.Script {
	s, err := moejs.CompileFunction("host-call", "fn, arg, ok, bad", helperBody, moejs.FunctionOptions{})
	if err != nil {
		panic(err)
	}
	return s
}()

// running is the plugin's runtime, started if it isn't.
func (p *Plugin) running() (*vm, error) {
	if v := p.current(); v != nil {
		return v, nil
	}
	p.starting.Lock()
	defer p.starting.Unlock()
	if v := p.current(); v != nil {
		return v, nil
	}
	// Not under mu: the top level may read its storage or settings.
	v, err := p.start()
	if err != nil {
		return nil, fmt.Errorf("starting plugin %s: %w", p.manifest.ID, err)
	}
	p.mu.Lock()
	p.vm = v
	p.mu.Unlock()
	return v, nil
}

func (p *Plugin) current() *vm {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.vm
}

// start runs the plugin, its module's top level too. The caller holds
// p.starting, and not p.mu, which the host functions take.
func (p *Plugin) start() (*vm, error) {
	// host.dataDir is there for the plugin to use, and for programs it starts.
	if err := os.MkdirAll(p.data, 0o700); err != nil {
		return nil, err
	}
	rt := moejs.NewRuntime(moejs.Options{MemoryLimit: memoryLimit})
	loop, err := eventloop.New(rt, eventloop.Options{
		OnError: func(err error) { log.Printf("plugin %s: %v", p.manifest.ID, err) },
	})
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	web := p.webOptions()
	p.mu.Unlock()
	api, err := webapi.Install(loop, web)
	if err != nil {
		return nil, err
	}
	v := &vm{rt: rt, loop: loop, api: api, programs: map[*program]bool{}}
	if err := p.installHost(v); err != nil {
		api.Close()
		return nil, err
	}
	if v.helper, err = rt.RunScript(helperScript); err != nil {
		api.Close()
		return nil, err
	}
	// A top level that awaits something finishes once the loop runs.
	if err := rt.Load(p.mod); err != nil && !errors.Is(err, moejs.ErrModulePending) {
		api.Close()
		return nil, err
	}
	if err := loop.Start(); err != nil {
		api.Close()
		return nil, err
	}
	return v, nil
}

// webOptions are what fetch may do: with no network permission it reaches
// nothing, and a plugin with programs reaches those on this machine, as that's
// how it talks to them. The caller holds p.mu.
func (p *Plugin) webOptions() webapi.Options {
	hosts := append([]string(nil), p.manifest.Permissions.Network...)
	if len(p.manifest.Permissions.Process) > 0 && p.granted {
		hosts = append(hosts, "127.0.0.1", "localhost")
	}
	if len(hosts) == 0 {
		return webapi.Options{}
	}
	opts := webapi.Options{Client: &http.Client{Timeout: 10 * time.Minute}}
	for _, h := range hosts {
		if h == "*" {
			return opts
		}
	}
	opts.Allow = webapi.AllowHosts(hosts...)
	return opts
}

type answer struct {
	json []byte
	err  error
}

// asError is the error a plugin threw, those the app acts on by what they are.
func asError(code, message string) error {
	var known error
	switch code {
	case "signIn":
		known = music.ErrSignIn
	case "setup":
		known = music.ErrSetup
	case "unavailable":
		known = music.ErrUnavailable
	case "unsupported":
		known = music.ErrUnsupported
	default:
		if message == "" {
			return errors.New("the plugin failed")
		}
		return errors.New(message)
	}
	return music.Fail(known, message)
}

// ask calls an export on the loop; the answer comes through got, once.
func (v *vm) ask(method string, params []byte, got func(answer)) {
	rt := v.rt
	fn, ok := rt.Export(method)
	if !ok {
		got(answer{err: music.ErrUnsupported})
		return
	}
	arg := moejs.Undefined()
	if params != nil {
		var err error
		if arg, err = rt.ParseJSON(params); err != nil {
			got(answer{err: err})
			return
		}
	}
	// The call's data goes once nothing is using it. It can't go any sooner, as
	// another call may be waiting on something with its own.
	done := func(a answer) {
		got(a)
		v.inFlight--
		v.loop.RunOnLoop(func(rt *moejs.Runtime) {
			if v.inFlight == 0 {
				rt.ReleaseCallData()
			}
		})
	}
	v.inFlight++
	okFn := rt.Function("ok", 1, func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
		out, err := rt.AppendJSON(nil, moejs.Arg(args, 0))
		done(answer{json: out, err: err})
		return moejs.Undefined(), nil
	})
	badFn := rt.Function("bad", 2, func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
		done(answer{err: asError(moejs.Arg(args, 0).String(), moejs.Arg(args, 1).String())})
		return moejs.Undefined(), nil
	})
	if _, err := rt.CallFunction(v.helper, moejs.Undefined(), fn, arg, okFn, badFn); err != nil {
		// Only an interrupt gets here: the helper catches what the plugin throws.
		done(answer{err: err})
	}
}

// call asks the plugin and decodes its answer into out, unless out is nil.
func (p *Plugin) call(method string, params, out any) error {
	return p.callWithin(context.Background(), callTimeout, method, params, out)
}

// callWithin is call that gives up when ctx ends, with its error, or after
// timeout.
func (p *Plugin) callWithin(ctx context.Context, timeout time.Duration, method string, params, out any) error {
	if !p.exports[method] {
		return music.ErrUnsupported
	}
	v, err := p.running()
	if err != nil {
		return err
	}
	var encoded []byte
	if params != nil {
		if encoded, err = json.Marshal(params); err != nil {
			return err
		}
	}
	ch := make(chan answer, 1)
	if !v.loop.RunOnLoop(func(*moejs.Runtime) {
		v.ask(method, encoded, func(a answer) { ch <- a })
	}) {
		p.drop(v)
		return fmt.Errorf("the plugin stopped; try again")
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case a := <-ch:
		if a.err != nil {
			return a.err
		}
		if out == nil {
			return nil
		}
		return json.Unmarshal(a.json, out)
	case <-ctx.Done():
		// The plugin's answer, when it comes, has nowhere to go: ch holds it.
		return ctx.Err()
	case <-timer.C:
		p.dropIfStuck(v)
		return fmt.Errorf("the plugin didn't answer %s in time", method)
	}
}

// dropIfStuck ends a runtime that takes on no more tasks, which code that never
// returns, or passing the memory limit, leaves it as. A plugin only waiting on
// something, and slow about it, goes on.
func (p *Plugin) dropIfStuck(v *vm) {
	alive := make(chan struct{})
	v.loop.RunOnLoop(func(*moejs.Runtime) { close(alive) })
	select {
	case <-alive:
	case <-time.After(stuckAfter):
		log.Printf("plugin %s: stuck; restarting it", p.manifest.ID)
		p.drop(v)
	}
}

func (p *Plugin) drop(v *vm) {
	p.mu.Lock()
	if p.vm == v {
		p.vm = nil
	}
	p.mu.Unlock()
	// Terminate waits for the loop, which the plugin may be keeping busy; it
	// interrupts it first, so this ends, but needn't hold up the caller.
	go v.close()
}

// SetSetting keeps the setting. A running plugin reads it through
// host.setting, and hears of it through setSetting if it exports that.
func (p *Plugin) SetSetting(key, value string) error {
	if err := p.keepSetting(key, value); err != nil {
		return err
	}
	p.mu.Lock()
	running := p.vm != nil
	p.mu.Unlock()
	if running && p.exports["setSetting"] {
		return p.call("setSetting", map[string]string{"key": key, "value": value}, nil)
	}
	return nil
}

var (
	_ music.Source   = (*Plugin)(nil)
	_ music.Recorder = (*Plugin)(nil)
	_ music.Prompter = (*Plugin)(nil)
	_ music.Canceler = (*Plugin)(nil)
)
