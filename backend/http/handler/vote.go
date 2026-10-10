package handler

import (
	"context"
	"errors"
	"time"

	microerr "go.unistack.org/micro/v3/errors"

	pb "github.com/VT0x00/vyborok/http/proto"
	"github.com/VT0x00/vyborok/internal/auth"
	"github.com/VT0x00/vyborok/internal/models"
	"github.com/VT0x00/vyborok/internal/vote"
	"github.com/google/uuid"
)

func (h *VyborokHandler) SubmitVote(ctx context.Context, req *pb.SubmitVoteReq, rsp *pb.SubmitVoteRsp) error {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		h.logger.Error("submit vote failed: bad id", "err", err, "poll_id", req.Id)
		return microerr.BadRequest("invalid_input", "invalid poll id")
	}

	var userID *uuid.UUID
	if uid, ok := auth.UserIDFromContext(ctx); ok {
		userID = &uid
	}

	answers := make([]vote.AnswerInput, 0, len(req.Answers))
	for _, a := range req.Answers {
		qid, err := uuid.Parse(a.QuestionId)
		if err != nil {
			h.logger.Error("submit vote failed: bad question id", "err", err)
			return microerr.BadRequest("invalid_input", "invalid question id")
		}
		optionIDs := make([]uuid.UUID, 0, len(a.OptionIds))
		for _, oid := range a.OptionIds {
			oidv, err := uuid.Parse(oid)
			if err != nil {
				h.logger.Error("submit vote failed: bad option id", "err", err)
				return microerr.BadRequest("invalid_input", "invalid option id")
			}
			optionIDs = append(optionIDs, oidv)
		}
		answers = append(answers, vote.AnswerInput{
			QuestionID: qid,
			OptionIDs:  optionIDs,
			Value:      int(a.Value),
			ValueSet:   a.ValueSet,
			Text:       a.Text,
		})
	}

	v, err := h.vote.Submit(ctx, vote.SubmitInput{
		PollID:  id,
		UserID:  userID,
		Answers: answers,
	})
	if err != nil {
		h.logger.Error("submit vote failed", "err", err, "poll_id", id)
		return mapVoteError(err)
	}

	rsp.Vote = toVote(v)
	return nil
}

func (h *VyborokHandler) HasVoted(ctx context.Context, req *pb.HasVotedReq, rsp *pb.HasVotedRsp) error {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		h.logger.Error("has voted failed: bad id", "err", err, "poll_id", req.Id)
		return microerr.BadRequest("invalid_input", "invalid poll id")
	}

	var userID *uuid.UUID
	if uid, ok := auth.UserIDFromContext(ctx); ok {
		userID = &uid
	}

	voted, err := h.vote.HasVoted(ctx, id, userID)
	if err != nil {
		h.logger.Error("has voted failed", "err", err, "poll_id", id)
		return mapVoteError(err)
	}

	rsp.HasVoted = voted
	return nil
}

// --- helpers ---

func toVote(v *models.Vote) *pb.Vote {
	if v == nil {
		return nil
	}
	out := &pb.Vote{
		Id:        v.ID.String(),
		PollId:    v.PollID.String(),
		CreatedAt: v.CreatedAt.Format(time.RFC3339),
		Answers:   make([]*pb.Answer, 0, len(v.Answers)),
	}
	if v.UserID != nil {
		out.UserId = v.UserID.String()
	}
	for i := range v.Answers {
		out.Answers = append(out.Answers, toAnswer(&v.Answers[i]))
	}
	return out
}

func toAnswer(a *models.Answer) *pb.Answer {
	out := &pb.Answer{
		QuestionId: a.QuestionID.String(),
		OptionIds:  make([]string, 0, len(a.OptionIDs)),
		Text:       a.Text,
	}
	for _, id := range a.OptionIDs {
		out.OptionIds = append(out.OptionIds, id.String())
	}
	if a.Value != nil {
		out.Value = int32(*a.Value)
		out.ValueSet = true
	}
	return out
}

func mapVoteError(err error) error {
	switch {
	case errors.Is(err, vote.ErrInvalidInput):
		return microerr.BadRequest("invalid_input", "%s", err.Error())
	case errors.Is(err, vote.ErrNotFound):
		return microerr.NotFound("not_found", "poll not found")
	case errors.Is(err, vote.ErrAuthRequired):
		return microerr.Unauthorized("unauthorized", "auth required")
	case errors.Is(err, vote.ErrPollClosed):
		return microerr.Conflict("poll_closed", "poll is closed")
	case errors.Is(err, vote.ErrAlreadyVoted):
		return microerr.Conflict("already_voted", "already voted")
	default:
		return microerr.InternalServerError("internal_error", "something went wrong")
	}
}
