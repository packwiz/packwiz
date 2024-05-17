FROM alpine:3.19.1

RUN apk update && apk upgrade --no-cache

RUN apk add curl && apk add unzip

#Waiting to see if the vuln fix makes it to the original maintainers repo
#RUN curl -L -o 'Linux64-bitx86.zip' https://nightly.link/packwiz/packwiz/workflows/go/main/Linux%2064-bit%20x86.zip
#RUN unzip 'Linux64-bitx86.zip'

COPY Linux64-bitx86.zip ./
RUN unzip 'Linux64-bitx86.zip'

RUN chmod 774 packwiz

VOLUME ["/data"]
WORKDIR /data

EXPOSE 8080

ENTRYPOINT [ "/packwiz", "server", "--port", "8080"]