package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/cvhariharan/flowctl/sdk/executor"
	"github.com/cvhariharan/flowctl/sdk/plugin/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

// grpcServer runs inside the plugin binary and implements proto.ExecutorPluginServer.
type grpcServer struct {
	proto.UnimplementedExecutorPluginServer
	impl      executor.ExecutorPlugin
	executors map[string]executor.Executor
}

func (s *grpcServer) GetName(_ context.Context, _ *emptypb.Empty) (*proto.GetNameResponse, error) {
	return &proto.GetNameResponse{Name: s.impl.GetName()}, nil
}

func (s *grpcServer) GetSchema(_ context.Context, _ *emptypb.Empty) (*proto.GetSchemaResponse, error) {
	schemaJSON, err := json.Marshal(s.impl.GetSchema())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema: %w", err)
	}
	return &proto.GetSchemaResponse{SchemaJson: schemaJSON}, nil
}

func (s *grpcServer) GetCapabilities(_ context.Context, _ *emptypb.Empty) (*proto.GetCapabilitiesResponse, error) {
	return &proto.GetCapabilitiesResponse{Capabilities: uint64(s.impl.GetCapabilities())}, nil
}

func (s *grpcServer) New(_ context.Context, req *proto.NewRequest) (*proto.NewResponse, error) {
	node := protoNodeToExecutor(req.Node)
	exec, err := s.impl.New(req.Name, node, req.ExecId)
	if err != nil {
		return &proto.NewResponse{Error: err.Error()}, nil
	}
	s.executors[req.ExecId] = exec
	return &proto.NewResponse{
		ExecutorId:   req.ExecId,
		ArtifactsDir: exec.GetArtifactsDir(),
	}, nil
}

func (s *grpcServer) Execute(req *proto.ExecuteRequest, stream proto.ExecutorPlugin_ExecuteServer) error {
	exec, ok := s.executors[req.ExecutorId]
	if !ok {
		return fmt.Errorf("executor %s not found", req.ExecutorId)
	}

	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()

	var wg sync.WaitGroup
	wg.Add(2)

	sendLog := func(r io.Reader, streamType proto.LogLine_Stream) {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				chunk := make([]byte, n)
				copy(chunk, buf[:n])
				_ = stream.Send(&proto.ExecuteResponse{
					Payload: &proto.ExecuteResponse_Log{
						Log: &proto.LogLine{Stream: streamType, Data: chunk},
					},
				})
			}
			if err != nil {
				return
			}
		}
	}

	go sendLog(stdoutR, proto.LogLine_STDOUT)
	go sendLog(stderrR, proto.LogLine_STDERR)

	sdkNodes := make([]executor.Node, len(req.ExecCtx.GetNodes()))
	for i, n := range req.ExecCtx.GetNodes() {
		sdkNodes[i] = protoNodeToExecutor(n)
	}

	execCtx := executor.ExecutionContext{
		WithConfig:    req.ExecCtx.GetWithConfig(),
		Inputs:        protoInputsToAny(req.ExecCtx.GetInputs()),
		Stdout:        stdoutW,
		Stderr:        stderrW,
		UserUUID:      req.ExecCtx.GetUserUuid(),
		NamespaceName: req.ExecCtx.GetNamespaceName(),
		APIKey:        req.ExecCtx.GetApiKey(),
		APIBaseURL:    req.ExecCtx.GetApiBaseUrl(),
		Nodes:         sdkNodes,
	}

	execResult, execErr := exec.Execute(stream.Context(), execCtx)

	stdoutW.Close()
	stderrW.Close()
	wg.Wait()

	result := &proto.Result{Outputs: execResult.Outputs, Globals: execResult.Globals}
	if execErr != nil {
		result.Error = execErr.Error()
	}
	return stream.Send(&proto.ExecuteResponse{
		Payload: &proto.ExecuteResponse_Result{Result: result},
	})
}

func protoNodeToExecutor(n *proto.Node) executor.Node {
	if n == nil {
		return executor.Node{}
	}
	return executor.Node{
		Hostname:       n.Hostname,
		Port:           int(n.Port),
		Username:       n.Username,
		ConnectionType: n.ConnectionType,
		OSFamily:       n.OsFamily,
		Auth: executor.NodeAuth{
			Method: n.AuthMethod,
			Key:    n.AuthKey,
		},
	}
}

func protoInputsToAny(m map[string]string) map[string]any {
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}
