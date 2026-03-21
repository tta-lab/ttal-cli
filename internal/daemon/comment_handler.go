package daemon

import (
	"context"
	"log"
	"time"

	"github.com/tta-lab/ttal-cli/internal/comment"
	"github.com/tta-lab/ttal-cli/internal/ent"
)

// toCommentEntries converts ent comment records to wire-format entries.
func toCommentEntries(comments []*ent.Comment) []CommentEntry {
	entries := make([]CommentEntry, 0, len(comments))
	for _, c := range comments {
		entries = append(entries, CommentEntry{
			Author:    c.Author,
			Body:      c.Body,
			Round:     c.Round,
			CreatedAt: c.CreatedAt.Format(time.RFC3339),
		})
	}
	return entries
}

func handleCommentAdd(svc *comment.Service, team string, req CommentAddRequest) CommentAddResponse {
	c, err := svc.Add(context.Background(), req.Target, req.Author, req.Body, team)
	if err != nil {
		log.Printf("[daemon] comment add failed: %v", err)
		return CommentAddResponse{OK: false, Error: err.Error()}
	}
	return CommentAddResponse{
		OK:    true,
		ID:    c.ID.String(),
		Round: c.Round,
	}
}

func handleCommentGet(svc *comment.Service, team string, req CommentGetRequest) CommentGetResponse {
	if req.Target == "" {
		return CommentGetResponse{OK: false, Error: "target is required"}
	}
	if req.Round < 1 {
		return CommentGetResponse{OK: false, Error: "round must be >= 1"}
	}
	comments, err := svc.GetByRound(context.Background(), req.Target, team, req.Round)
	if err != nil {
		log.Printf("[daemon] comment get failed: %v", err)
		return CommentGetResponse{OK: false, Error: err.Error()}
	}
	return CommentGetResponse{OK: true, Comments: toCommentEntries(comments)}
}

func handleCommentList(svc *comment.Service, team string, req CommentListRequest) CommentListResponse {
	comments, err := svc.List(context.Background(), req.Target, team)
	if err != nil {
		log.Printf("[daemon] comment list failed: %v", err)
		return CommentListResponse{OK: false, Error: err.Error()}
	}
	return CommentListResponse{OK: true, Comments: toCommentEntries(comments)}
}
