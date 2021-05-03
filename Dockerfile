FROM golang:1.14

RUN apt update \
    && apt-get -y install zip g++ 

COPY . /root/sunBob-2/super-signature

WORKDIR /root/sunBob-2/super-signature/zsign
RUN g++ *.cpp common/*.cpp -lcrypto -std=c++11 -o zsign
RUN sudo cp ./zsign /usr/bin/zsign

ENV GOPROXY https://goproxy.cn,direct
WORKDIR /root/sunBob-2/super-signature
RUN go env -w GO111MODULE=on \
    && go build -o "super-signature" .

EXPOSE 4443
ENTRYPOINT ["./super-signature"]

# 《====可在app.ini配置数据库服务器====》
# 《====直接运行以下命令====》
# 编译
# docker build -t super-signature:v1 .
# 查看生成镜像
# docker images
# 启动容器
# docker run -it -d --name super-signature -p 4443:4443 super-signature:v1
# 可进入容器 docker exec -it super-signature bash
# 查看日志 docker logs -f 容器ID
# 验证服务是否启动 ps -A | grep super-signature
# 浏览器访问 https://localhost:4443/docs/index.html