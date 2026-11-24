package netease

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"netease-music-mygo/internal/music"
)

// episodesPerPage is how many of a podcast's episodes come at a time.
const episodesPerPage = 30

func parseShowID(id string) (int64, error) {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("netease: no such podcast %q", id)
	}
	return n, nil
}

// Shows lists the podcasts the listener subscribes to, the latest first.
func (Source) Shows() ([]music.Show, error) {
	pods, err := SubscribedPodcasts()
	if err != nil {
		return nil, fmt.Errorf("netease: %w", err)
	}
	return ToShows(pods), nil
}

// ToEpisode is an episode of pod as the app's track: its audio's ID under the
// episode's name, and the day it came out. An episode with no audio, which NetEase
// lists but cannot play, is unavailable.
func ToEpisode(p Program, pod *Podcast) music.Track {
	mt := ToTrack(p.Track(pod))
	mt.Published = time.UnixMilli(p.CreateTime).Format("2006-01-02")
	if p.MainSong.ID == 0 {
		mt.Unavailable = true
	}
	return mt
}

// Show answers a podcast and a page of its episodes from offset, the newest
// first.
func (Source) Show(id string, offset int) (*music.ShowPage, error) {
	pid, err := parseShowID(id)
	if err != nil {
		return nil, err
	}
	var (
		wg       sync.WaitGroup
		pod      *Podcast
		programs []Program
		more     bool
		podErr   error
		listErr  error
	)
	wg.Add(2)
	go func() { defer wg.Done(); pod, podErr = PodcastDetail(pid) }()
	go func() { defer wg.Done(); programs, more, listErr = PodcastPrograms(pid, episodesPerPage, offset) }()
	wg.Wait()
	if podErr != nil {
		return nil, fmt.Errorf("netease: %w", podErr)
	}
	if listErr != nil {
		return nil, fmt.Errorf("netease: %w", listErr)
	}
	page := &music.ShowPage{Show: ToShow(*pod), More: more && len(programs) > 0}
	for _, p := range programs {
		page.Episodes = append(page.Episodes, ToEpisode(p, pod))
	}
	return page, nil
}

// SubscribeShow subscribes the listener to a podcast, or ends it.
func (Source) SubscribeShow(id string, on bool) error {
	pid, err := parseShowID(id)
	if err != nil {
		return err
	}
	return SubscribePodcast(pid, on)
}

// PodcastPicks are the podcasts NetEase picks to listen to, and the most
// popular, a page at a time from offset: what its podcasts tab lists besides the
// listener's subscriptions, which no other source has.
func (Source) PodcastPicks() ([]music.Show, error) {
	pods, err := RecommendedPodcasts()
	if err != nil {
		return nil, fmt.Errorf("netease: %w", err)
	}
	return ToShows(pods), nil
}

// HotPodcasts lists the most popular podcasts from offset, and whether there
// are more.
func (Source) HotPodcasts(offset int) ([]music.Show, bool, error) {
	pods, more, err := HotPodcasts(episodesPerPage, offset)
	if err != nil {
		return nil, false, fmt.Errorf("netease: %w", err)
	}
	return ToShows(pods), more, nil
}
