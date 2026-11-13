package accounts

import (
	"context"
	"errors"
	"log/slog"

	accountsv1 "github.com/gopherbazaar/ledger/gen/ledger/accounts/v1"
	"github.com/gopherbazaar/ledger/internal/grpcx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// errorDomain identifies this service in ErrorInfo details.
const errorDomain = "accounts.ledger.gopherbazaar.dev"

type GRPCServer struct {
	accountsv1.UnimplementedAccountsServiceServer

	store  *Store
	logger *slog.Logger
}

func NewGRPCServer(store *Store, logger *slog.Logger) *GRPCServer {
	return &GRPCServer{store: store, logger: logger}
}

func (s *GRPCServer) CreateAccount(ctx context.Context, req *accountsv1.CreateAccountRequest) (*accountsv1.CreateAccountResponse, error) {
	if req.GetOwner() == "" {
		return nil, status.Error(codes.InvalidArgument, "owner must be provided")
	}
	a, err := s.store.CreateAccount(ctx, req.GetOwner(), req.GetOpeningDeposit())
	if err != nil {
		return nil, s.toStatus(err)
	}
	return &accountsv1.CreateAccountResponse{Account: toProto(a)}, nil
}

func (s *GRPCServer) GetAccount(ctx context.Context, req *accountsv1.GetAccountRequest) (*accountsv1.GetAccountResponse, error) {
	a, err := s.store.GetAccount(ctx, req.GetId())
	if err != nil {
		return nil, s.toStatus(err)
	}
	return &accountsv1.GetAccountResponse{Account: toProto(a)}, nil
}

func (s *GRPCServer) Debit(ctx context.Context, req *accountsv1.DebitRequest) (*accountsv1.DebitResponse, error) {
	if err := s.store.Debit(ctx, req.GetAccountId(), req.GetTransferId(), req.GetAmount()); err != nil {
		return nil, s.toStatus(err)
	}
	return &accountsv1.DebitResponse{}, nil
}

func (s *GRPCServer) Credit(ctx context.Context, req *accountsv1.CreditRequest) (*accountsv1.CreditResponse, error) {
	if err := s.store.Credit(ctx, req.GetAccountId(), req.GetTransferId(), req.GetAmount()); err != nil {
		return nil, s.toStatus(err)
	}
	return &accountsv1.CreditResponse{}, nil
}

func (s *GRPCServer) Refund(ctx context.Context, req *accountsv1.RefundRequest) (*accountsv1.RefundResponse, error) {
	if err := s.store.Refund(ctx, req.GetAccountId(), req.GetTransferId(), req.GetAmount()); err != nil {
		return nil, s.toStatus(err)
	}
	return &accountsv1.RefundResponse{}, nil
}

func (s *GRPCServer) GetTotalBalance(ctx context.Context, _ *accountsv1.GetTotalBalanceRequest) (*accountsv1.GetTotalBalanceResponse, error) {
	total, err := s.store.TotalBalance(ctx)
	if err != nil {
		return nil, s.toStatus(err)
	}
	return &accountsv1.GetTotalBalanceResponse{Total: total}, nil
}

func (s *GRPCServer) Reconcile(ctx context.Context, _ *accountsv1.ReconcileRequest) (*accountsv1.ReconcileResponse, error) {
	mismatches, err := s.store.Reconcile(ctx)
	if err != nil {
		return nil, s.toStatus(err)
	}
	resp := &accountsv1.ReconcileResponse{}
	for _, m := range mismatches {
		resp.Mismatches = append(resp.Mismatches, &accountsv1.Mismatch{
			AccountId:     m.AccountID,
			Balance:       m.Balance,
			LedgerBalance: m.LedgerBalance,
		})
	}
	return resp, nil
}

func (s *GRPCServer) toStatus(err error) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return grpcx.Error(codes.NotFound, err.Error(), errorDomain, "ACCOUNT_NOT_FOUND")
	case errors.Is(err, ErrInsufficientFunds):
		return grpcx.Error(codes.FailedPrecondition, err.Error(), errorDomain, "INSUFFICIENT_FUNDS")
	case errors.Is(err, ErrInvalidAmount):
		return grpcx.Error(codes.InvalidArgument, err.Error(), errorDomain, "INVALID_AMOUNT")
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, err.Error())
	}
	s.logger.Error("internal error", "error", err)
	return status.Error(codes.Internal, "internal error")
}

func toProto(a Account) *accountsv1.Account {
	return &accountsv1.Account{
		Id:         a.ID,
		Owner:      a.Owner,
		Balance:    a.Balance,
		Currency:   a.Currency,
		CreateTime: timestamppb.New(a.CreatedAt),
	}
}
