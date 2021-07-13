FROM public.ecr.aws/amazonlinux/amazonlinux:latest

COPY amazon-ssm-agent.rpm amazon-ssm-agent.json start-agent.sh /

RUN set -e; \
   if [[ "$(arch)" == "x86_64" ]]; then \
     export ARCH='amd64'; \
   elif [[ "$(arch)" == "aarch64" ]]; then \
     export ARCH='arm64'; \
   else \
     echo >&2 "error: unsupported architecture '$apkArch'"; \
     exit 1; \
   fi; \
   \
   yum install -y /amazon-ssm-agent.rpm jq; \
   mv /amazon-ssm-agent.json /etc/amazon/ssm/; \
   rm /amazon-ssm-agent.rpm; \
   yum clean all; \
   rm -rf /var/cache/yum;

CMD ["/start-agent.sh"]