# Debian bookworm: its glibc targets the ARMv8.0 baseline, which the static-pie
# linux/arm64 build embeds. Do not move the arm64 build to a libc built for
# ARMv8.1+ (e.g. Amazon Linux 2023), see aws/amazon-ssm-agent#683.
FROM public.ecr.aws/docker/library/golang:1.26-bookworm

RUN apt -y update && apt -y upgrade && apt -y install rpm tar gzip wget zip && apt clean all

RUN mkdir /amazon-ssm-agent
WORKDIR /amazon-ssm-agent
