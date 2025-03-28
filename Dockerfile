# 分两个阶段构建：build阶段用于编译生成二进制文件
# 使用官方基础镜像：https://hub.docker.com/_/golang/tags
# 注意基础镜像中也有一些环境变量，可以直接继承，比如 GOPATH
# 不指定镜像仓库，默认是 docker.io/library/
#FROM golang:1.21.3-alpine3.18 AS build
FROM docker.1ms.run/library/golang:1.21.3-alpine3.18 AS build

# 添加标签信息
LABEL stage=build

# 添加一个构建参数，使用方法： --build-arg GOPROXY=https://proxy.example.com
ARG GOPROXY
ENV GOPROXY=${GOPROXY:-https://goproxy.cn,https://goproxy.io,direct}
ENV CGO_ENABLED 0

# apk 是 alpine 的包管理工具，这里用于安装时区数据，还可以安装其他软件包和工具
RUN apk update --no-cache && apk add --no-cache tzdata

# 镜像中设置编译阶段工作目录
WORKDIR /build
# 将 Dockfile 所在文件夹下的文件复制到 /build
# ADD 比 COPY 功能更多支持从 URL 下载文件
ADD go.mod .
ADD go.sum .
# 执行 shell 命令下载依赖
RUN go mod download
# 拷贝 Dockfile 所在文件夹下的文件复制到 /build
COPY . .

#RUN go build -ldflags="-s -w" -o /app/signal_server src/components/signal/main.go
RUN go build -o /app/signal_server src/components/signal/main.go

# 第二阶段，
# scratch镜像的特点是没有任何依赖，只有二进制文件，体积非常小，但是因为缺少工具和操作系统文件，调试会比较困难
#FROM scratch
#FROM alpine3.18
FROM docker.1ms.run/library/alpine:3.18

EXPOSE 18900

# 从 build 阶段复制文件
#COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /usr/share/zoneinfo/Asia/Shanghai /usr/share/zoneinfo/Asia/Shanghai
ENV TZ Asia/Shanghai

WORKDIR /app
COPY --from=build /app/signal_server /app/signal_server

CMD ["./signal_server"]
