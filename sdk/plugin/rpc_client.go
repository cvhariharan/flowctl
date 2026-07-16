package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/cvhariharan/flowctl/sdk/executor"
	"github.com/cvhariharan/flowctl/sdk/plugin/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

// grpcClient runs in the host process and implements executor.ExecutorPlugin.
type grpcClient struct {
	client proto.ExecutorPluginClient
}

func (c *grpcClient) GetName() string {
	resp, err := c.client.GetName(context.Background(), &emptypb.Empty{})
	if err != nil {
		return ""
	}
	return resp.Name
}

func (c *grpcClient) GetSchema() interface{} {
	resp, err := c.client.GetSchema(context.Background(), &emptypb.Empty{})
	if err != nil {
		return nil
	}
	var schema interface{}
	if err := json.Unmarshal(resp.SchemaJson, &schema); err != nil {
		return nil
	}
	return schema
}

func (c *grpcClient) GetCapabilities() executor.Capability {
	resp, err := c.client.GetCapabilities(context.Background(), &emptypb.Empty{})
	if err != nil {
		return 0
	}
	return executor.Capability(resp.Capabilities)
}

func (c *grpcClient) New(name string, node executor.Node, execID string) (executor.Executor, error) {
	req := &proto.NewRequest{
		Name:   name,
		ExecId: execID,
		Node: &proto.Node{
			Hostname:       node.Hostname,
			Port:           int32(node.Port),
			Username:       node.Username,
			AuthMethod:     node.Auth.Method,
			AuthKey:        node.Auth.Key,
			ConnectionType: node.ConnectionType,
			OsFamily:       node.OSFamily,
		},
	}
	resp, err := c.client.New(context.Background(), req)
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("%s", resp.Error)
	}
	return &grpcRemoteExecutor{
		client:       c.client,
		executorID:   resp.ExecutorId,
		artifactsDir: resp.ArtifactsDir,
	}, nil
}

// grpcRemoteExecutor implements executor.Executor and delegates Execute over gRPC streaming.
type grpcRemoteExecutor struct {
	client       proto.ExecutorPluginClient
	executorID   string
	artifactsDir string
}

func (r *grpcRemoteExecutor) GetArtifactsDir() string {
	return r.artifactsDir
}

func (r *grpcRemoteExecutor) Close() error {
	return nil
}

func (r *grpcRemoteExecutor) Execute(ctx context.Context, execCtx executor.ExecutionContext) (executor.ExecutionResult, error) {
	protoNodes := make([]*proto.Node, len(execCtx.Nodes))
	for i, n := range execCtx.Nodes {
		protoNodes[i] = &proto.Node{
			Hostname:       n.Hostname,
			Port:           int32(n.Port),
			Username:       n.Username,
			AuthMethod:     n.Auth.Method,
			AuthKey:        n.Auth.Key,
			ConnectionType: n.ConnectionType,
			OsFamily:       n.OSFamily,
		}
	}

	req := &proto.ExecuteRequest{
		ExecutorId: r.executorID,
		ExecCtx: &proto.ExecutionContext{
			WithConfig:    execCtx.WithConfig,
			Inputs:        anyInputsToProto(execCtx.Inputs),
			UserUuid:      execCtx.UserUUID,
			NamespaceName: execCtx.NamespaceName,
			ApiKey:        execCtx.APIKey,
			ApiBaseUrl:    execCtx.APIBaseURL,
			Nodes:         protoNodes,
		},
	}

	stream, err := r.client.Execute(ctx, req)
	if err != nil {
		return executor.ExecutionResult{}, err
	}

	var (
		result  executor.ExecutionResult
		execErr error
	)

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return executor.ExecutionResult{}, err
		}

		switch p := msg.Payload.(type) {
		case *proto.ExecuteResponse_Log:
			if p.Log.Stream == proto.LogLine_STDOUT && execCtx.Stdout != nil {
				execCtx.Stdout.Write(p.Log.Data)
			} else if p.Log.Stream == proto.LogLine_STDERR && execCtx.Stderr != nil {
				execCtx.Stderr.Write(p.Log.Data)
			}
		case *proto.ExecuteResponse_Result:
			result.Outputs = p.Result.Outputs
			result.Globals = p.Result.Globals
			if p.Result.Error != "" {
				execErr = fmt.Errorf("%s", p.Result.Error)
			}
		}
	}

	return result, execErr
}

func anyInputsToProto(m map[string]any) map[string]string {
	result := make(map[string]string, len(m))
	for k, v := range m {
		switch s := v.(type) {
		case string:
			result[k] = s
		default:
			b, _ := json.Marshal(v)
			result[k] = string(b)
		}
	}
	return result
}
