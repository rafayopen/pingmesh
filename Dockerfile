##
# Build a docker image to run the pingmesh application.
# The CloudWatch version requires /bin/sh, so build from Alpine.
##
FROM alpine:3.4

RUN apk update && apk add ca-certificates && /bin/rm -rf /var/cache/apk/*
RUN apk add --no-cache curl

ADD cmd/pingmesh/pingmesh.exe /usr/local/bin/pingmesh

# Include -c option to publish to CloudWatch.  You must also set the following
# environment variables: AWS_REGION, AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY.
# Include -v option to turn on verbose logging.
ENTRYPOINT [ "/usr/local/bin/pingmesh", "-c", "-s", "80", "-r", "443"  ]
