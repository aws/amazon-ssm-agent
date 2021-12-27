FROM golang:1.17

RUN apt -y update && apt -y upgrade && apt -y install rpm tar gzip wget zip && apt clean all

RUN mkdir /amazon-ssm-agent
WORKDIR /amazon-ssm-agent
