package transfers

import (
	"context"
	"errors"
	"log/slog"

	transfersv1 "github.com/gopherbazaar/ledger/gen/ledger/transfers/v1"
	"github.com/gopherbazaar/ledger/internal/grpcx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const errorDomain = "transfers.ledger.gopherbazaar.dev"

type GRPCServer struct {
	transfersv1.UnimplementedTransfersServiceServer

	service *Service
	store   *Store
	logger  *slog.Logger
}

func NewGRPCServer(service *Service, store *Store, logger *slog.Logger) *GRPCServer {
	return &GRPCServer{service: service, store: store, logger: logger}
}

func (s *GRPCServer) CreateTransfer(ctx context.Context, req *transfersv1.CreateTransferRequest) (*transfersv1.CreateTransferResponse, error) {
	t, err := s.service.Transfer(ctx, req.GetFromAccountId(), req.GetToAccountId(), req.GetAmount())
	if err != nil && !errors.Is(err, ErrOutcomeUnknown) {
		return nil, s.toStatus(ctx, err)
	}
	return &transfersv1.CreateTransferResponse{Transfer: toProto(t)}, nil
}

func (s *GRPCServer) GetTransfer(ctx context.Context, req *transfersv1.GetTransferRequest) (*transfersv1.GetTransferResponse, error) {
	t, err := s.store.Get(ctx, req.GetId())
	if err != nil {
		return nil, s.toStatus(ctx, err)
	}
	return &transfersv1.GetTransferResponse{Transfer: toProto(t)}, nil
}

func (s *GRPCServer) GetTransferStats(ctx context.Context, req *transfersv1.GetTransferStatsRequest) (*transfersv1.GetTransferStatsResponse, error) {
	st, err := s.store.Stats(ctx)
	if err != nil {
		return nil, s.toStatus(ctx, err)
	}
	res := &transfersv1.GetTransferStatsResponse{
		ByStatus: make(map[string]int64, len(st.ByStatus)),
		InFlight: st.InFlight,
	}
	for status, n := range st.ByStatus {
		res.ByStatus[string(status)] = n
	}
	return res, nil
}

func (s *GRPCServer) toStatus(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, ErrInvalidAmount):
		return grpcx.Error(codes.InvalidArgument, err.Error(), errorDomain, "INVALID_AMOUNT")
	case errors.Is(err, ErrSameAccount):
		return grpcx.Error(codes.InvalidArgument, err.Error(), errorDomain, "SAME_ACCOUNT")
	case errors.Is(err, ErrAccountNotFound):
		return grpcx.Error(codes.NotFound, err.Error(), errorDomain, "ACCOUNT_NOT_FOUND")
	case errors.Is(err, ErrNotFound):
		return grpcx.Error(codes.NotFound, err.Error(), errorDomain, "TRANSFER_NOT_FOUND")
	case errors.Is(err, ErrInsufficientFunds):
		return grpcx.Error(codes.FailedPrecondition, err.Error(), errorDomain, "INSUFFICIENT_FUNDS")
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, err.Error())
	case status.Code(err) == codes.DeadlineExceeded, status.Code(err) == codes.Unavailable:
		return grpcx.Error(codes.Unavailable, "accounts service unavailable", errorDomain, "DEPENDENCY_UNAVAILABLE")
	}
	s.logger.ErrorContext(ctx, "internal error", "error", err)
	return status.Error(codes.Internal, "internal error")
}

var statusToProto = map[Status]transfersv1.TransferStatus{
	StatusPending:   transfersv1.TransferStatus_TRANSFER_STATUS_PENDING,
	StatusDebited:   transfersv1.TransferStatus_TRANSFER_STATUS_DEBITED,
	StatusCompleted: transfersv1.TransferStatus_TRANSFER_STATUS_COMPLETED,
	StatusFailed:    transfersv1.TransferStatus_TRANSFER_STATUS_FAILED,
	StatusRefunded:  transfersv1.TransferStatus_TRANSFER_STATUS_REFUNDED,
}

func toProto(t Transfer) *transfersv1.Transfer {
	return &transfersv1.Transfer{
		Id:            t.ID,
		FromAccountId: t.FromAccountID,
		ToAccountId:   t.ToAccountID,
		Amount:        t.Amount,
		Status:        statusToProto[t.Status],
	}
}
