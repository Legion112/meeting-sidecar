# CUDA Whisper build for meeting-sidecar.
# Override paths if needed: make CUDA_HOME=/opt/cuda WHISPER_CPP=$HOME/src/whisper.cpp

HOME        ?= $(shell echo $$HOME)
GO_BIN      ?= $(HOME)/sdk/go1.27.0/bin
CUDA_HOME   ?= $(HOME)/sdk/cuda-13.0
WHISPER_CPP ?= $(HOME)/sdk/whisper.cpp
WHISPER_BUILD ?= $(WHISPER_CPP)/build_go

export PATH            := $(GO_BIN):$(CUDA_HOME)/bin:$(PATH)
export CUDA_PATH       := $(CUDA_HOME)
export CUDA_HOME
export PKG_CONFIG_PATH := /usr/lib/x86_64-linux-gnu/pkgconfig
export C_INCLUDE_PATH  := $(WHISPER_CPP)/include:$(WHISPER_CPP)/ggml/include
export LIBRARY_PATH    := $(WHISPER_BUILD)/src:$(WHISPER_BUILD)/ggml/src:$(WHISPER_BUILD)/ggml/src/ggml-cuda:$(CUDA_HOME)/lib64:$(CUDA_HOME)/targets/x86_64-linux/lib
export LD_LIBRARY_PATH := $(CUDA_HOME)/lib64:$(CUDA_HOME)/targets/x86_64-linux/lib:$(LD_LIBRARY_PATH)
export CGO_ENABLED     := 1

GO       ?= go
BIN      ?= meeting-sidecar
PKG      ?= ./cmd/meeting-sidecar
CUDA_LIB64   := $(CUDA_HOME)/lib64
CUDA_TARGET  := $(CUDA_HOME)/targets/x86_64-linux/lib
EXTLD        := -Wl,-rpath,$(CUDA_LIB64) -Wl,-rpath,$(CUDA_TARGET) -lggml-cuda -lcudart -lcublas -lcuda -lculibos
LDFLAGS      := -ldflags "-extldflags '$(EXTLD)'"

.PHONY: all build test run run-debug clean check-deps list-monitors

all: build

check-deps:
	@test -x "$(CUDA_HOME)/bin/nvcc" || { echo "missing nvcc at $(CUDA_HOME)/bin/nvcc"; exit 1; }
	@test -f "$(WHISPER_BUILD)/src/libwhisper.a" || { echo "missing libwhisper.a; build whisper.cpp with GGML_CUDA=ON under $(WHISPER_BUILD)"; exit 1; }
	@test -f "$(WHISPER_BUILD)/ggml/src/ggml-cuda/libggml-cuda.a" || { echo "missing libggml-cuda.a under $(WHISPER_BUILD)"; exit 1; }

build: check-deps
	$(GO) build $(LDFLAGS) -o $(BIN) $(PKG)

test: check-deps
	$(GO) test ./internal/... -covermode=atomic $(LDFLAGS)

run: build
	./$(BIN) -config config.yaml

run-debug: build
	./$(BIN) -debug -config config.yaml

list-monitors: build
	./$(BIN) -list-monitors

clean:
	rm -f $(BIN)
