# Demo plugin

Upstream's demo plugin (egoist/mygo-music, `examples/demo-plugin`), which does
everything a plugin can, with made-up music. The tests run it, and it is the way
to try plugins by hand:

1. Copy this folder into `plugins/` in the app's user data folder (on macOS,
   `~/Library/Application Support/<the app's name>/plugins/demo`), and start the
   app. Switch to 演示 with the button over the search field.
2. `tone.mp3` is the song every track plays: its `stream` answers a `file://` URL
   joined from the folder it is in (`host.dir`), which may hold spaces and other
   scripts, as the app's own folder's name does. Leave it in the folder: without
   it a song says its audio file is missing.
