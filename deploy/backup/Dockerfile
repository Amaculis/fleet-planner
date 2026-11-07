# Backup runner: the same Postgres major as the db service (pg_dump must not be older
# than the server) plus age for encryption. Pinned by digest, like the other images.
FROM postgres:17.11-alpine@sha256:3540c0362c809df084ffac0ae69fe794515f9de92692f02178bcdce0ef0302ac
RUN apk add --no-cache age
# Owned by the runtime user so a fresh named volume inherits writable ownership.
RUN mkdir -p /backups && chown postgres:postgres /backups
COPY backup.sh upload.sh /usr/local/bin/
RUN chmod 0555 /usr/local/bin/backup.sh /usr/local/bin/upload.sh
USER postgres
ENTRYPOINT ["/usr/local/bin/backup.sh"]
