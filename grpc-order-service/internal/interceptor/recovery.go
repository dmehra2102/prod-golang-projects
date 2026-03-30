package interceptor

import (
	"context"
	"fmt"
	"runtime/debug"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func UnaryRecovery(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if p := recover(); p != nil {
				stack := debug.Stack()
				logger.Error("panic in unary handler",
					zap.String("method", info.FullMethod),
					zap.Any("panic", p),
					zap.ByteString("stack", stack),
				)

				err = status.Errorf(codes.Internal, "internal server error: %v", fmt.Sprintf("%v", p))
			}
		}()

		return handler(ctx, req)
	}
}

func StreamRecovery(logger *zap.Logger) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if p := recover(); p != nil {
				stack := debug.Stack()
				logger.Error("panic in stream handler",
					zap.String("method", info.FullMethod),
					zap.Any("panic", p),
					zap.ByteString("stack", stack),
				)

				err = status.Errorf(codes.Internal, "internal server error : %v", fmt.Sprintf("%v", p))
			}
		}()

		return handler(srv, ss)
	}
}
