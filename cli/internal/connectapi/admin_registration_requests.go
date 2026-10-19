package connectapi

import (
	"context"

	"connectrpc.com/connect"
	"github.com/chattocorp/chatto/cli/internal/core"
	adminv1 "github.com/chattocorp/chatto/cli/internal/pb/chatto/admin/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultRegistrationRequestLimit = 20
	maxRegistrationRequestLimit     = 100
)

type adminRegistrationRequestService struct{ api *API }

func (s *adminRegistrationRequestService) ListRegistrationRequests(ctx context.Context, req *connect.Request[adminv1.ListRegistrationRequestsRequest]) (*connect.Response[adminv1.ListRegistrationRequestsResponse], error) {
	caller, err := requireCaller(ctx)
	if err != nil {
		return nil, err
	}
	views, err := s.api.core.ListRegistrationRequests(ctx, caller.UserID)
	if err != nil {
		return nil, err
	}
	limit, offset := apiPagination(req.Msg.GetPage(), defaultRegistrationRequestLimit, maxRegistrationRequestLimit)
	total := len(views)
	if offset > total {
		offset = total
	}
	end := min(offset+limit, total)
	result := make([]*adminv1.PendingRegistration, 0, end-offset)
	for _, view := range views[offset:end] {
		result = append(result, apiRegistrationRequest(view))
	}
	return connect.NewResponse(&adminv1.ListRegistrationRequestsResponse{
		RegistrationRequests: result,
		Page:                 apiPageInfo(total, end < total),
	}), nil
}

func (s *adminRegistrationRequestService) ApproveRegistrationRequest(ctx context.Context, req *connect.Request[adminv1.ApproveRegistrationRequestRequest]) (*connect.Response[adminv1.ApproveRegistrationRequestResponse], error) {
	caller, err := requireCaller(ctx)
	if err != nil {
		return nil, err
	}
	view, err := s.api.core.ApproveRegistrationRequest(ctx, caller.UserID, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminv1.ApproveRegistrationRequestResponse{Login: view.Login}), nil
}

func (s *adminRegistrationRequestService) DenyRegistrationRequest(ctx context.Context, req *connect.Request[adminv1.DenyRegistrationRequestRequest]) (*connect.Response[adminv1.DenyRegistrationRequestResponse], error) {
	caller, err := requireCaller(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.api.core.DenyRegistrationRequest(ctx, caller.UserID, req.Msg.GetId()); err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminv1.DenyRegistrationRequestResponse{}), nil
}

func apiRegistrationRequest(view core.RegistrationRequestView) *adminv1.PendingRegistration {
	return &adminv1.PendingRegistration{
		Id:        view.ID,
		Login:     view.Login,
		Email:     view.Email,
		Note:      view.Note,
		CreatedAt: timestamppb.New(view.CreatedAt),
	}
}
