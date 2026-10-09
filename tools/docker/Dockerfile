# Dev image for botify-labs/v8go: Go 1.27, LLVM 21 (v8go_source mode and the
# prebuilt bridge), gcc (consumer mode and the V8 9.0 baseline), and the
# benchmark/API tools. Nothing is installed on the host.
FROM golang:1.27-bookworm

RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      lsb-release wget gnupg software-properties-common \
      python3 python3-yaml rsync git binutils \
 && wget -qO /tmp/llvm.sh https://apt.llvm.org/llvm.sh \
 && bash /tmp/llvm.sh 21 \
 && apt-get install -y --no-install-recommends libclang-rt-21-dev clang-format-21 \
 && rm -rf /var/lib/apt/lists/* /tmp/llvm.sh

RUN go install golang.org/x/perf/cmd/benchstat@latest \
 && go install golang.org/x/exp/cmd/apidiff@latest

RUN git config --system --add safe.directory '*'

# Login shells (bash -l) reset PATH on Debian; restore the Go paths.
RUN echo 'export PATH=/usr/local/go/bin:/go/bin:$PATH' > /etc/profile.d/00-go.sh
COPY profile.sh /etc/profile.d/v8go.sh
ENV GOFLAGS=-buildvcs=false
WORKDIR /src/v8go
