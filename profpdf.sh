go tool pprof -pdf  cpu.out > cpu.pdf
go tool pprof -pdf -alloc_space main.test mem.out > memSpace.pdf
go tool pprof -pdf -alloc_objects main.test mem.out > memObjects.pdf
go tool pprof -pdf -inuse_space main.test mem.out > memInUseSpace.pdf
go tool pprof -pdf -inuse_objects main.test mem.out > memInUseObjects.pdf