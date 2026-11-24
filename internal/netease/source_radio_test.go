package netease

import (
	"testing"
)

// The personal FM's three modes are stations, and the next songs of one are
// what NetEase's radio gives in that mode, as tracks with their own record; a
// station that is none is refused before NetEase is asked.
func TestSourceStationNext(t *testing.T) {
	asked, done := serveByPath(t, map[string]string{
		"/api/v1/radio/get":    `{"code":200,"data":[{"id":11},{"id":12}]}`,
		"/api/v3/song/detail":  `{"code":200,"songs":[{"id":11,"name":"甲","dt":1000},{"id":12,"name":"乙","dt":2000,"fee":1}],"privileges":[]}`,
		"/api/radio/trash/add": `{"code":200}`,
	})
	defer done()
	src := Source{}
	stations, _ := src.Stations()
	if len(stations) != 3 || stations[0].ID != FMDefault || stations[1].ID != FMFamiliar || stations[2].ID != FMExplore {
		t.Fatalf("stations %+v", stations)
	}
	tracks, err := src.StationNext(FMFamiliar)
	if err != nil || len(tracks) != 2 || tracks[0].ID != "11" || tracks[1].Name != "乙" || !tracks[1].Unavailable {
		t.Fatalf("%+v, %v", tracks, err)
	}
	if got := asked(); len(got) < 1 || got[0].path != "/api/v1/radio/get" || got[0].data["mode"] != FMFamiliar {
		t.Errorf("asked %+v", got)
	}
	n := len(asked())
	if _, err := src.StationNext("OTHER"); err == nil || len(asked()) != n {
		t.Errorf("a station that is none: %v", err)
	}
	if err := src.TrashFM("11"); err != nil {
		t.Fatal(err)
	}
	last := asked()[len(asked())-1]
	if last.path != "/api/radio/trash/add" || last.data["songId"] != "11" {
		t.Errorf("trashed %+v", last)
	}
}
