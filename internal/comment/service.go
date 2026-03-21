package comment

import (
	"context"

	"entgo.io/ent/dialect/sql"
	"github.com/tta-lab/ttal-cli/internal/ent"
	entcomment "github.com/tta-lab/ttal-cli/internal/ent/comment"
)

// Service wraps the ent client for comment persistence.
type Service struct {
	client *ent.Client
}

// NewService creates a new comment Service.
func NewService(client *ent.Client) *Service {
	return &Service{client: client}
}

// Add creates a comment with an auto-incremented round number.
func (s *Service) Add(ctx context.Context, target, author, body, team string) (*ent.Comment, error) {
	round, err := s.CurrentRound(ctx, target, team)
	if err != nil {
		return nil, err
	}

	return s.client.Comment.Create().
		SetTarget(target).
		SetAuthor(author).
		SetBody(body).
		SetTeam(team).
		SetRound(round + 1).
		Save(ctx)
}

// List returns comments for target+team ordered by created_at ASC.
func (s *Service) List(ctx context.Context, target, team string) ([]*ent.Comment, error) {
	return s.client.Comment.Query().
		Where(
			entcomment.Target(target),
			entcomment.Team(team),
		).
		Order(entcomment.ByCreatedAt()).
		All(ctx)
}

// GetByRound returns comments for target+team filtered to a specific round.
func (s *Service) GetByRound(ctx context.Context, target, team string, round int) ([]*ent.Comment, error) {
	return s.client.Comment.Query().
		Where(
			entcomment.Target(target),
			entcomment.Team(team),
			entcomment.Round(round),
		).
		Order(entcomment.ByCreatedAt()).
		All(ctx)
}

// CurrentRound returns the latest round number for target+team (0 if none).
func (s *Service) CurrentRound(ctx context.Context, target, team string) (int, error) {
	comments, err := s.client.Comment.Query().
		Where(
			entcomment.Target(target),
			entcomment.Team(team),
		).
		Order(entcomment.ByRound(sql.OrderDesc())).
		Limit(1).
		All(ctx)
	if err != nil {
		return 0, err
	}
	if len(comments) == 0 {
		return 0, nil
	}
	return comments[0].Round, nil
}
