package upstreamreader

import (
	"context"
	"fmt"

	reconciliationcontract "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/reconciliation/contract"
	siteconnectiondomain "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/siteconnection/domain"
	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/upstream"
)

type ConnectionProvider interface {
	GetByID(id uint) (*siteconnectiondomain.Connection, error)
	GetAdapter(connection *siteconnectiondomain.Connection) (upstream.Adapter, error)
}

type Reader struct {
	connections ConnectionProvider
}

var _ reconciliationcontract.UpstreamOrderProvider = (*Reader)(nil)

type session struct {
	adapter upstream.Adapter
}

var _ reconciliationcontract.UpstreamOrderReader = (*session)(nil)

func New(connections ConnectionProvider) *Reader {
	if connections == nil {
		panic("reconciliation upstream reader: connections is nil")
	}
	return &Reader{connections: connections}
}

func (r *Reader) Open(connectionID uint) (reconciliationcontract.UpstreamOrderReader, error) {
	connection, err := r.connections.GetByID(connectionID)
	if err != nil {
		return nil, fmt.Errorf("get connection: %w", err)
	}
	adapter, err := r.connections.GetAdapter(connection)
	if err != nil {
		return nil, fmt.Errorf("get adapter: %w", err)
	}
	return &session{adapter: adapter}, nil
}

func (s *session) Get(ctx context.Context, upstreamOrderID uint) (*reconciliationcontract.UpstreamOrder, error) {
	detail, err := s.adapter.GetOrder(ctx, upstreamOrderID)
	if err != nil {
		return nil, err
	}
	return &reconciliationcontract.UpstreamOrder{Status: detail.Status, Amount: detail.Amount}, nil
}
