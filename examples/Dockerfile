FROM golang:1.18 as chisel-builder

RUN git clone https://github.com/canonical/chisel
WORKDIR chisel
RUN go build -o /chisel-bin ./cmd/chisel/


FROM ubuntu:22.04 as builder

WORKDIR /
COPY --from=chisel-builder /chisel-bin chisel
COPY install.sh .

RUN apt update && \
	apt install -y ca-certificates file

RUN mkdir /rootfs && \
	CHISEL_DPKG_STATUS_FILE=status ./install.sh --release "ubuntu-22.04" --root /rootfs dotnet-runtime-6.0_libs


FROM scratch
COPY --from=builder /rootfs /
COPY --from=builder /status /var/lib/dpkg/status
