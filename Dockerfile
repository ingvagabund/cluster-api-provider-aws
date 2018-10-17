# Copyright 2018 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Reproducible builder image
FROM openshift/origin-release:golang-1.10 as builder

# Workaround a bug in imagebuilder (some versions) where this dir will not be auto-created.
RUN mkdir -p /go/src/sigs.k8s.io/cluster-api-provider-aws
WORKDIR /go/src/sigs.k8s.io/cluster-api-provider-aws

# This expects that the context passed to the docker build command is
# the cluster-api-provider-aws directory.
# e.g. docker build -t <tag> -f <this_Dockerfile> <path_to_cluster-api-aws>
COPY pkg/    pkg/
COPY cmd/    cmd/
COPY vendor/ vendor/


RUN GOPATH="/go" CGO_ENABLED=0 GOOS=linux go build -o /go/bin/machine-controller-manager -ldflags '-extldflags "-static"' sigs.k8s.io/cluster-api-provider-aws/cmd/manager
RUN GOPATH="/go" CGO_ENABLED=0 GOOS=linux go build -o /go/bin/manager -ldflags '-extldflags "-static"' sigs.k8s.io/cluster-api-provider-aws/vendor/sigs.k8s.io/cluster-api/cmd/manager

# Final container
FROM openshift/origin-base
RUN yum install -y ca-certificates openssh

COPY --from=builder /go/bin/manager /go/bin/machine-controller-manager /
