package netease

import (
	"fmt"
	"strconv"

	"netease-music-mygo/internal/music"
)

// commentThread is what NetEase calls the comments of a track: a song's by its
// number, an episode's by its program's.
func commentThread(id string) (string, error) {
	audio, program, err := parseTrackID(id)
	if err != nil {
		return "", err
	}
	if program != 0 {
		return programThread(program), nil
	}
	return "R_SO_4_" + strconv.FormatInt(audio, 10), nil
}

func toComment(cm Comment) music.Comment {
	out := music.Comment{
		ID:        strconv.FormatInt(cm.ID, 10),
		Author:    cm.User.Nickname,
		AvatarURL: imageURL(cm.User.AvatarURL),
		Text:      cm.Content,
		Time:      cm.Time,
		Likes:     cm.LikedCount,
		Liked:     cm.Liked,
		Location:  cm.IPLocation.Location,
	}
	if len(cm.BeReplied) > 0 {
		r := cm.BeReplied[0]
		out.Reply = &music.Reply{Author: r.User.Nickname, Text: r.Content}
	}
	return out
}

// commentsPerPage is how many comments NetEase is asked for at a time.
const commentsPerPage = 30

// Comments lists the latest comments of a track from offset, and on the first
// page its hot ones as well.
func (Source) Comments(trackID string, offset int) (*music.CommentPage, error) {
	thread, err := commentThread(trackID)
	if err != nil {
		return nil, err
	}
	res, err := ThreadComments(thread, offset, commentsPerPage)
	if err != nil {
		return nil, fmt.Errorf("netease: %w", err)
	}
	page := &music.CommentPage{Total: res.Total, More: res.More}
	for _, cm := range res.Hot {
		page.Hot = append(page.Hot, toComment(cm))
	}
	for _, cm := range res.Latest {
		page.Comments = append(page.Comments, toComment(cm))
	}
	return page, nil
}

// LikeComment likes a comment of a track for the listener, or unlikes it.
func (Source) LikeComment(trackID, commentID string, on bool) error {
	thread, err := commentThread(trackID)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(commentID, 10, 64)
	if err != nil || id <= 0 {
		return fmt.Errorf("netease: no such comment %q", commentID)
	}
	return LikeComment(thread, id, on)
}
