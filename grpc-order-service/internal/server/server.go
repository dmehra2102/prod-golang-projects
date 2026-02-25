package server

import (
	"context"
	"fmt"
	"io"
	"slices"
	"sync"
	"time"

	pb "github.com/dmehra2102/prod-golang-projects/grpc-order-service/gen/order/v1"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type OrderServer struct {
	pb.UnimplementedOrderServiceServer

	mu     sync.RWMutex
	orders map[string]*pb.Order
	logger *zap.Logger
}

func New(logger *zap.Logger) *OrderServer {
	return &OrderServer{
		orders: make(map[string]*pb.Order),
		logger: logger,
	}
}

func validateItems(items []*pb.LineItem) error {
	if len(items) == 0 {
		return status.Error(codes.InvalidArgument, "order must contain at least one item")
	}
	for i, item := range items {
		if item.ProductId == "" {
			return status.Errorf(codes.InvalidArgument, "item[%d]: product_id is required", i)
		}
		if item.Quantity <= 0 {
			return status.Errorf(codes.InvalidArgument, "item[%d]: quantity must be positive", i)
		}
		if item.UnitPriceCents <= 0 {
			return status.Errorf(codes.InvalidArgument, "item[%d]: unit_price_cents must be positive", i)
		}
	}
	return nil
}

func totalCents(items []*pb.LineItem) int64 {
	var total int64
	for _, item := range items {
		total += int64(item.Quantity) * item.UnitPriceCents
	}
	return total
}

func (s *OrderServer) CreateOrder(tx context.Context, req *pb.CreateOrderRequest) (*pb.CreateOrderResponse, error) {
	if req.CustomerId == "" {
		return nil, status.Error(codes.InvalidArgument, "customer_id is required")
	}
	if err := validateItems(req.Items); err != nil {
		return nil, err
	}

	now := timestamppb.Now()
	order := &pb.Order{
		Id:         fmt.Sprintf("ord_%s", uuid.New().String()[:8]),
		CustomerId: req.CustomerId,
		Items:      req.Items,
		TotalCents: totalCents(req.Items),
		Status:     pb.OrderStatus_ORDER_STATUS_PENDING,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	s.mu.Lock()
	s.orders[order.Id] = order
	s.mu.Unlock()

	s.logger.Info("order created", zap.String("order_id", order.Id), zap.String("customer_id", order.CustomerId))
	return &pb.CreateOrderResponse{Order: order}, nil
}

func (s *OrderServer) GetOrder(ctx context.Context, req *pb.GetOrderRequest) (*pb.GetOrderResponse, error) {
	if req.OrderId == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}

	s.mu.RLock()
	order, ok := s.orders[req.OrderId]
	s.mu.RUnlock()

	if !ok {
		return nil, status.Errorf(codes.NotFound, "order %q not found", req.OrderId)
	}

	return &pb.GetOrderResponse{Order: order}, nil
}

var validTransitions = map[pb.OrderStatus][]pb.OrderStatus{
	pb.OrderStatus_ORDER_STATUS_PENDING:   {pb.OrderStatus_ORDER_STATUS_CONFIRMED, pb.OrderStatus_ORDER_STATUS_CANCELLED},
	pb.OrderStatus_ORDER_STATUS_CONFIRMED: {pb.OrderStatus_ORDER_STATUS_SHIPPED, pb.OrderStatus_ORDER_STATUS_CANCELLED},
	pb.OrderStatus_ORDER_STATUS_SHIPPED:   {pb.OrderStatus_ORDER_STATUS_DELIVERED},
	pb.OrderStatus_ORDER_STATUS_DELIVERED: {},
	pb.OrderStatus_ORDER_STATUS_CANCELLED: {},
}

func isValidTransition(from, to pb.OrderStatus) bool {
	return slices.Contains(validTransitions[from], to)
}

func (s *OrderServer) UpdateOrderStatus(ctx context.Context, req *pb.UpdateOrderStatusRequest) (*pb.UpdateOrderStatusResponse, error) {
	if req.OrderId == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}
	if req.NewStatus == pb.OrderStatus_ORDER_STATUS_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "new_status is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	order, ok := s.orders[req.OrderId]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "order %q not found", req.OrderId)
	}

	if !isValidTransition(order.Status, req.NewStatus) {
		return nil, status.Errorf(
			codes.FailedPrecondition,
			"cannot transition order from %s to %s",
			order.Status, req.NewStatus,
		)
	}

	order.Status = req.NewStatus
	order.UpdatedAt = timestamppb.Now()

	return &pb.UpdateOrderStatusResponse{Order: order}, nil
}

func (s *OrderServer) WatchShipping(req *pb.WatchShippingRequest, stream pb.OrderService_WatchShippingServer) error {
	if req.OrderId == "" {
		return status.Error(codes.InvalidArgument, "order_id is required")
	}

	s.mu.RLock()
	_, ok := s.orders[req.OrderId]
	s.mu.RUnlock()

	if !ok {
		return status.Errorf(codes.NotFound, "order %q not found", req.OrderId)
	}

	// Simulate shipping events. In production, this would subscribe to a
	// message broker (Kafka, Pub/Sub) and fan out events as they arrive.
	events := []struct {
		carrier  string
		location string
		desc     string
		delay    time.Duration
	}{
		{"FedEx", "Warehouse", "Order picked and packed", 200 * time.Millisecond},
		{"FedEx", "Memphis Hub", "In transit", 400 * time.Millisecond},
		{"FedEx", "Local Facility", "Out for delivery", 600 * time.Millisecond},
		{"FedEx", "Destination", "Delivered", 800 * time.Millisecond},
	}

	for _, e := range events {
		select {
		case <-stream.Context().Done():
			return status.FromContextError(stream.Context().Err()).Err()
		case <-time.After(e.delay):
		}

		if err := stream.Send(&pb.ShippingEvent{
			OrderId:     req.OrderId,
			Carrier:     e.carrier,
			TrackingId:  fmt.Sprintf("TRK-%s", req.OrderId),
			Location:    e.location,
			Description: e.desc,
			OccurredAt:  timestamppb.Now(),
		}); err != nil {
			return status.Errorf(codes.Internal, "failed to send event: %v", err)
		}
	}

	return nil // stream closed gracefully by returning nil
}

func (s *OrderServer) BulkCreateOrders(stream pb.OrderService_BulkCreateOrdersServer) error {
	var (
		created    int32
		failed     int32
		totalValue int64
		orderIDs   []string
	)

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			// Client finished sending
			return stream.SendAndClose(&pb.BulkCreateOrdersResponse{
				OrdersCreated:   created,
				OrdersFailed:    failed,
				TotalValueCents: totalValue,
				OrderIds:        orderIDs,
			})
		}
		if err != nil {
			return status.Errorf(codes.Internal, "recv error: %v", err)
		}

		if stream.Context().Err() != nil {
			return status.FromContextError(stream.Context().Err()).Err()
		}

		// Validate and create each order
		if req.CustomerId == "" || len(req.Items) == 0 {
			failed++
			continue
		}
		if err := validateItems(req.Items); err != nil {
			failed++
			continue
		}

		now := timestamppb.Now()
		order := &pb.Order{
			Id:         fmt.Sprintf("ord_%s", uuid.New().String()[:8]),
			CustomerId: req.CustomerId,
			Items:      req.Items,
			TotalCents: totalCents(req.Items),
			Status:     pb.OrderStatus_ORDER_STATUS_PENDING,
			CreatedAt:  now,
			UpdatedAt:  now,
		}

		s.mu.Lock()
		s.orders[order.Id] = order
		s.mu.Unlock()

		orderIDs = append(orderIDs, order.Id)
		totalValue += order.TotalCents
		created++
	}
}

func (s *OrderServer) OrderChannel(stream pb.OrderService_OrderChannelServer) error {
	for {
		cmd, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return status.Errorf(codes.Internal, "recv error: %v", err)
		}
		if stream.Context().Err() != nil {
			return status.FromContextError(stream.Context().Err()).Err()
		}

		switch c := cmd.Command.(type) {
		case *pb.OrderCommand_Create:
			resp, err := s.CreateOrder(stream.Context(), c.Create)
			if err != nil {
				_ = stream.Send(&pb.OrderEvent{
					Event: &pb.OrderEvent_ErrorMessage{ErrorMessage: err.Error()},
				})
				continue
			}
			_ = stream.Send(&pb.OrderEvent{
				Event: &pb.OrderEvent_OrderCreated{OrderCreated: resp.Order},
			})
		case *pb.OrderCommand_Update:
			resp, err := s.UpdateOrderStatus(stream.Context(), c.Update)
			if err != nil {
				_ = stream.Send(&pb.OrderEvent{
					Event: &pb.OrderEvent_ErrorMessage{ErrorMessage: err.Error()},
				})
				continue
			}
			_ = stream.Send(&pb.OrderEvent{
				Event: &pb.OrderEvent_OrderUpdated{OrderUpdated: resp.Order},
			})

		case *pb.OrderCommand_Get:
			resp, err := s.GetOrder(stream.Context(), c.Get)
			if err != nil {
				_ = stream.Send(&pb.OrderEvent{
					Event: &pb.OrderEvent_ErrorMessage{ErrorMessage: err.Error()},
				})
				continue
			}
			_ = stream.Send(&pb.OrderEvent{
				Event: &pb.OrderEvent_OrderFetched{OrderFetched: resp.Order},
			})

		default:
			_ = stream.Send(&pb.OrderEvent{
				Event: &pb.OrderEvent_ErrorMessage{ErrorMessage: "unknown command type"},
			})
		}
	}
}
