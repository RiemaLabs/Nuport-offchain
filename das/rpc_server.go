package das

var (
// rpcStoreRequestGauge     = metrics.NewRegisteredGauge("nubit/das/rpc/store/requests", nil)
// rpcStoreSuccessGauge     = metrics.NewRegisteredGauge("nubit/das/rpc/store/success", nil)
// rpcStoreFailureGauge     = metrics.NewRegisteredGauge("nubit/das/rpc/store/failure", nil)
// rpcStoreStoredBytesGauge = metrics.NewRegisteredGauge("nubit/das/rpc/store/bytes", nil)
// //rpcStoreDurationHistogram = metrics.NewRegisteredHistogram("nubit/das/rpc/store/duration", nil, metrics.NewboundedHistogramSample())

// rpcReadRequestGauge   = metrics.NewRegisteredGauge("nubit/das/rpc/read/requests", nil)
// rpcReadSuccessGauge   = metrics.NewRegisteredGauge("nubit/das/rpc/read/success", nil)
// rpcReadFailureGauge   = metrics.NewRegisteredGauge("nubit/das/rpc/read/failure", nil)
// rpcReadReadBytesGauge = metrics.NewRegisteredGauge("nubit/das/rpc/read/bytes", nil)
// //rpcReadDurationHistogram = metrics.NewRegisteredHistogram("nubit/das/rpc/read/duration", nil, metrics.NewBoundedHistogramSample())

// rpcProofRequestGauge = metrics.NewRegisteredGauge("nubit/das/rpc/proof/requests", nil)
// rpcProofSuccessGauge = metrics.NewRegisteredGauge("nubit/das/rpc/proof/success", nil)
// rpcProofFailureGauge = metrics.NewRegisteredGauge("nubit/das/rpc/proof/failure", nil)
// rpcProofBytesGauge   = metrics.NewRegisteredGauge("nubit/das/rpc/proof/bytes", nil)

// rpcProofDurationHistogram = metrics.NewRegisteredHistogram("nubit/das/rpc/proof/duration", nil, metrics.)
)

// type NubitDASRPCServer struct {
// 	nubitReader NubitReader
// 	nubitWriter NubitWriter
// }

// func StartDASRPCServer(ctx context.Context, addr string, portNum uint64, rpcServerTimeouts genericconf.HTTPServerTimeoutConfig, rpcServerBodyLimit int, nubitReader NubitReader, nubitWriter NubitWriter) (*http.Server, error) {
// 	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", addr, portNum))
// 	if err != nil {
// 		return nil, err
// 	}
// 	return StartNubitDASRPCServerOnListener(ctx, listener, rpcServerTimeouts, rpcServerBodyLimit, nubitReader, nubitWriter)
// }

// func StartNubitDASRPCServerOnListener(ctx context.Context, listener net.Listener, rpcServerTimeouts genericconf.HTTPServerTimeoutConfig, rpcServerBodyLimit int, nubitReader NubitReader, nubitWriter NubitWriter) (*http.Server, error) {
// 	if nubitWriter == nil {
// 		return nil, errors.New("No writer backend was configured for Nubit DAS RPC server. Please setup a node and ensure a connections is being established")
// 	}
// 	rpcServer := rpc.NewServer()
// 	if rpcServerBodyLimit > 0 {
// 		rpcServer.SetHTTPBodyLimit(rpcServerBodyLimit)
// 	}
// 	err := rpcServer.RegisterName("nubit", &NubitDASRPCServer{
// 		nubitReader: nubitReader,
// 		nubitWriter: nubitWriter,
// 	})
// 	if err != nil {
// 		return nil, err
// 	}

// 	srv := &http.Server{
// 		Handler:           rpcServer,
// 		ReadTimeout:       rpcServerTimeouts.ReadTimeout,
// 		ReadHeaderTimeout: rpcServerTimeouts.ReadHeaderTimeout,
// 		WriteTimeout:      rpcServerTimeouts.WriteTimeout,
// 		IdleTimeout:       rpcServerTimeouts.IdleTimeout,
// 	}

// 	go func() {
// 		err := srv.Serve(listener)
// 		if err != nil {
// 			return
// 		}
// 	}()
// 	go func() {
// 		<-ctx.Done()
// 		_ = srv.Shutdown(context.Background())
// 	}()
// 	return srv, nil
// }

// func (serv *NubitDASRPCServer) Store(ctx context.Context, message hexutil.Bytes) ([]byte, error) {
// 	log.Trace("nubitDasRpc.NubitDASRPCServer.Store", "message", pretty.FirstFewBytes(message), "message length", len(message), "this", serv)
// 	rpcStoreRequestGauge.Inc(1)
// 	//	start := time.Now()
// 	success := false
// 	defer func() {
// 		if success {
// 			rpcStoreSuccessGauge.Inc(1)
// 		} else {
// 			rpcStoreFailureGauge.Inc(1)
// 		}
// 		//	rpcStoreDurationHistogram.Update(time.Since(start).Nanoseconds())
// 	}()

// 	result, err := serv.nubitWriter.Store(ctx, message)
// 	if err != nil {
// 		return nil, err
// 	}
// 	rpcStoreStoredBytesGauge.Inc(int64(len(message)))
// 	success = true
// 	return result, nil
// }

// func (serv *NubitDASRPCServer) Read(ctx context.Context, blobPointer *BlobPointer) (*ReadResult, error) {
// 	log.Trace("nubitDasRpc.NubitDASRPCServer.Read", "blob pointer", blobPointer, "this", serv)
// 	rpcReadRequestGauge.Inc(1)
// 	//start := time.Now()
// 	success := false
// 	defer func() {
// 		if success {
// 			rpcReadSuccessGauge.Inc(1)
// 		} else {
// 			rpcReadFailureGauge.Inc(1)
// 		}
// 		//rpcReadDurationHistogram.Update(time.Since(start).Nanoseconds())
// 	}()

// 	result, err := serv.nubitReader.Read(ctx, blobPointer)
// 	if err != nil {
// 		return nil, err
// 	}
// 	rpcReadReadBytesGauge.Inc(int64(len(result.Message)))
// 	success = true
// 	return result, nil
// }

// func (serv *NubitDASRPCServer) GetProof(ctx context.Context, msg []byte) ([]byte, error) {
// 	log.Trace("nubitDasRpc.NubitDASRPCServer.GetProof", "message", pretty.FirstFewBytes(msg), "message length", len(msg), "this", serv)
// 	rpcProofRequestGauge.Inc(1)
// 	//start := time.Now()
// 	success := false
// 	defer func() {
// 		if success {
// 			rpcProofSuccessGauge.Inc(1)
// 		} else {
// 			rpcProofFailureGauge.Inc(1)
// 		}
// 		//	rpcProofDurationHistogram.Update(time.Since(start).Nanoseconds())
// 	}()

// 	proof, err := serv.nubitReader.GetProof(ctx, msg)
// 	if err != nil {
// 		return nil, err
// 	}
// 	rpcProofBytesGauge.Inc(int64(len(proof)))
// 	success = true
// 	return proof, nil
// }

// func (serv *NubitDASRPCServer) Verify(ctx context.Context, msg []byte) (bool, error) {
// 	log.Trace("nubitDasRpc.NubitDASRPCServer.Verify", "message", pretty.FirstFewBytes(msg), "message length", len(msg), "this", serv)
// 	return serv.nubitReader.Verify(ctx, msg)
// }
