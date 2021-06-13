FROM registry.fedoraproject.org/fedora:34
RUN dnf install -y git go ninja-build unzip && dnf clean all
RUN \
  curl -o gn.zip -L https://chrome-infra-packages.appspot.com/dl/gn/gn/linux-amd64/+/latest && \
  unzip gn.zip gn && install -Dm 755 gn /usr/local/bin/gn && rm -f gn.zip gn
COPY entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
