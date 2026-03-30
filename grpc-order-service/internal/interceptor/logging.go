package interceptor

import (
	"context"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

func codeToLevel(code codes.Code) zapcore.Level {
	switch code {
	case codes.OK:
		return zapcore.InfoLevel
	case codes.NotFound, codes.AlreadyExists, codes.InvalidArgument, codes.FailedPrecondition,
		codes.Unauthenticated, codes.PermissionDenied, codes.Canceled:
		return zapcore.WarnLevel
	default:
		return zapcore.ErrorLevel
	}
}

func UnaryLogging(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		start := time.Now()
		resp, err = handler(ctx, req)
		duration := time.Since(start)

		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}

		peerAddr := "unknown"
		if p, ok := peer.FromContext(ctx); ok {
			peerAddr = p.Addr.String()
		}

		level := codeToLevel(code)
		logger.Check(level, "grpc call").Write(
			zap.String("method", info.FullMethod),
			zap.String("peer", peerAddr),
			zap.Duration("duration", duration),
			zap.String("code", code.String()),
			zap.Error(err),
		)

		return resp, err
	}
}

func StreamLogging(logger *zap.Logger) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		logger.Info("stream started", zap.String("method", info.FullMethod))

		err := handler(srv, ss)
		duration := time.Since(start)

		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}

		level := codeToLevel(code)
		logger.Check(level, "stream ended").Write(
			zap.String("method", info.FullMethod),
			zap.Duration("duration", duration),
			zap.String("code", code.String()),
			zap.Error(err),
		)
		return err
	}
}
