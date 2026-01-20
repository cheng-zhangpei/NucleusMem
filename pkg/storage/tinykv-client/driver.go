package tinykv_client

import (
	"time"
)

// TinyKVDriver this driver is to get grpc connection and the TSO service
type TinyKVDriver struct {
	//client kvrpcpb.TinyKvClient
	// tsoClient ... (你需要一个能获取 TSO 的客户端，或者 mock 一个)
}

//func NewTinyKVDriver(addr string) (*TinyKVDriver, error) {
//	conn, err := grpc.Dial(addr, grpc.WithInsecure())
//	if err != nil {
//		return nil, err
//	}
//	return &TinyKVDriver{
//		client: kvrpcpb.NewTinyKvClient(conn),
//	}, nil
//}

// 辅助：获取 StartTS (模拟)
func (d *TinyKVDriver) getStartTS() uint64 {
	// 实际应该调 PD，这里先用时间戳模拟
	return uint64(time.Now().UnixNano())
}
