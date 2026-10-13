package grpc

import roomv1 "booking/contracts/room/v1"

type Server struct {
	roomv1.UnimplementedRoomServiceServer
}
