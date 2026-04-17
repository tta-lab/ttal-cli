package daemon

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/tta-lab/ttal-cli/internal/comment"
	"github.com/tta-lab/ttal-cli/internal/ent"
	"github.com/tta-lab/ttal-cli/internal/gitprovider"
	"github.com/tta-lab/ttal-cli/internal/project"
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

func handleCommentAdd(svc *comment.Service, team, commentSync string, req CommentAddRequest) CommentAddResponse {
	c, err := svc.Add(context.Background(), req.Target, req.Author, req.Body, team)
	if err != nil {
		log.Printf("[daemon] comment add failed: %v", err)
		return CommentAddResponse{OK: false, Error: err.Error()}
	}

	if commentSync == "pr" && req.PRIndex > 0 {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[daemon] mirror comment to PR: panic: %v", r)
				}
			}()
			mirrorCommentToPR(req, c.Round)
		}()
	}

	return CommentAddResponse{
		OK:    true,
		ID:    c.ID.String(),
		Round: c.Round,
	}
}

func mirrorCommentToPR(req CommentAddRequest, round int) {
	// Forgejo requires Host; skip mirroring gracefully if not provided.
	if req.ProviderType == "forgejo" && req.Host == "" {
		log.Printf("[daemon] mirror comment to PR: skipping (forgejo host not set)")
		return
	}

	token := project.ResolveGitHubToken(req.ProjectAlias)
	provider, err := gitprovider.NewProviderByNameWithToken(req.ProviderType, token, req.Host)
	if err != nil {
		log.Printf("[daemon] mirror comment to PR: create provider: %v", err)
		return
	}
	body := fmt.Sprintf("**%s** (round %d):\n\n%s", req.Author, round, req.Body)
	if _, err := provider.CreateComment(req.Owner, req.Repo, req.PRIndex, body); err != nil {
		log.Printf("[daemon] mirror comment to PR #%d: %v", req.PRIndex, err)
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
