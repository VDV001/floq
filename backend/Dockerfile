FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY server /server
COPY migrations/ /migrations/
EXPOSE 8080
CMD ["/server"]
