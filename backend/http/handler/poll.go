package handler

import (
	"context"
	"errors"
	"time"

	microerr "go.unistack.org/micro/v3/errors"

	pb "github.com/VT0x00/vyborok/http/proto"
	"github.com/VT0x00/vyborok/internal/auth"
	"github.com/VT0x00/vyborok/internal/models"
	"github.com/VT0x00/vyborok/internal/poll"
)

func (h *VyborokHandler) CreatePoll(ctx context.Context, req *pb.CreatePollReq, rsp *pb.PollRsp) error {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return microerr.Unauthorized("unauthorized", "auth required")
	}

	questions := make([]poll.CreateQuestionInput, 0, len(req.Questions))
	for _, q := range req.Questions {
		questions = append(questions, poll.CreateQuestionInput{
			Type:          q.Type,
			Title:         q.Title,
			Description:   q.Description,
			ScaleMin:      int(q.ScaleMin),
			ScaleMax:      int(q.ScaleMax),
			TextMaxLength: int(q.TextMaxLength),
			Options:       optionTexts(q.Options),
		})
	}

	p, err := h.poll.Create(ctx, poll.CreateInput{
		UserID:      userID,
		Title:       req.Title,
		Description: req.Description,
		Anonymous:   req.Anonymous,
		Questions:   questions,
	})
	if err != nil {
		h.logger.Error("create poll failed", "err", err, "user_id", userID)
		return mapPollError(err)
	}

	rsp.Poll = toPoll(p)
	return nil
}

func (h *VyborokHandler) GetPoll(ctx context.Context, req *pb.GetPollReq, rsp *pb.PollRsp) error {
	return microerr.InternalServerError("not_implemented", "GetPoll is not implemented yet")
}

func (h *VyborokHandler) ListPolls(ctx context.Context, req *pb.ListPollsReq, rsp *pb.ListPollsRsp) error {
	return microerr.InternalServerError("not_implemented", "ListPolls is not implemented yet")
}

func (h *VyborokHandler) UpdatePoll(ctx context.Context, req *pb.UpdatePollReq, rsp *pb.PollRsp) error {
	return microerr.InternalServerError("not_implemented", "UpdatePoll is not implemented yet")
}

func (h *VyborokHandler) DeletePoll(ctx context.Context, req *pb.DeletePollReq, rsp *pb.DeletePollRsp) error {
	return microerr.InternalServerError("not_implemented", "DeletePoll is not implemented yet")
}

func (h *VyborokHandler) ClosePoll(ctx context.Context, req *pb.ClosePollReq, rsp *pb.PollRsp) error {
	return microerr.InternalServerError("not_implemented", "ClosePoll is not implemented yet")
}

// --- helpers ---

func optionTexts(in []*pb.CreateOptionReq) []string {
	out := make([]string, 0, len(in))
	for _, o := range in {
		out = append(out, o.Text)
	}
	return out
}

func toPoll(p *models.Poll) *pb.Poll {
	if p == nil {
		return nil
	}
	out := &pb.Poll{
		Id:          p.ID.String(),
		UserId:      p.UserID.String(),
		Title:       p.Title,
		Description: p.Description,
		Status:      string(p.Status),
		Anonymous:   p.Anonymous,
		CreatedAt:   p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   p.UpdatedAt.Format(time.RFC3339),
		Questions:   make([]*pb.Question, 0, len(p.Questions)),
	}
	for i := range p.Questions {
		out.Questions = append(out.Questions, toQuestion(&p.Questions[i]))
	}
	return out
}

func toQuestion(q *models.Question) *pb.Question {
	out := &pb.Question{
		Id:          q.ID.String(),
		Type:        string(q.Type),
		Title:       q.Title,
		Description: q.Description,
		Position:    int32(q.Position),
		Options:     make([]*pb.Option, 0, len(q.Options)),
	}
	if q.ScaleMin != nil {
		out.ScaleMin = int32(*q.ScaleMin)
	}
	if q.ScaleMax != nil {
		out.ScaleMax = int32(*q.ScaleMax)
	}
	if q.TextMaxLength != nil {
		out.TextMaxLength = int32(*q.TextMaxLength)
	}
	for i := range q.Options {
		o := &q.Options[i]
		out.Options = append(out.Options, &pb.Option{
			Id:       o.ID.String(),
			Text:     o.Text,
			Position: int32(o.Position),
		})
	}
	return out
}

func mapPollError(err error) error {
	switch {
	case errors.Is(err, poll.ErrInvalidInput):
		return microerr.BadRequest("invalid_input", "%s", err.Error())
	case errors.Is(err, poll.ErrNotFound):
		return microerr.NotFound("not_found", "poll not found")
	case errors.Is(err, poll.ErrForbidden):
		return microerr.Forbidden("forbidden", "forbidden")
	case errors.Is(err, poll.ErrPollClosed):
		return microerr.BadRequest("poll_closed", "poll is closed")
	default:
		return microerr.InternalServerError("internal_error", "something went wrong")
	}
}
