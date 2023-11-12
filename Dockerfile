# First stage: Build the books binary
FROM golang:alpine AS builder

# Set the working directory and pull in the source
WORKDIR /build
COPY . /build

# Install dependencies.
RUN apk add --no-cache make build-base

# Install Mage
RUN go install github.com/magefile/mage@latest

# Create a non-root user for later stages.
RUN echo "nobody:x:65534:65534:nobody:/:/sbin/nologin" > ./passwd \
    && echo "nobody:x:65534:" > ./group                           \
    && chmod 644 ./passwd                                         \
    && chmod 644 ./group

# Build the application.
RUN mage

# Set executable permissions on the built binary.
RUN chmod +x bin/books

# Extract libraries to be copied into the Scratch image later.
RUN ldd ./bin/books | tr -s '[:blank:]' '\n' | grep '^/' | xargs -I % sh -c 'mkdir -p $(dirname deps%); cp % deps%;'

# Prepare a default configuration and template files.
RUN mkdir -p config config/cache books_root \
    && mv cmd/books/example_config.toml config/config.toml \
    && mv templates config/ \
    && chown -R 65534:65534 config books_root
RUN echo -e "# Used by docker for storing books.\nroot = \"/books_root\"" > ./config/config.toml

# Second stage: Create a minimal runtime environment
FROM scratch

# Set the working directory for the running container.
WORKDIR /config

# Copy over user/group definitions.
COPY --from=builder /build/passwd /build/group /etc/

# Now, the built binary and its dependencies.
COPY --from=builder /build/bin/books /build/deps /

# Grab our default config folder.
COPY --from=builder /build/config /config
COPY --from=builder /build/books_root /books_root

# Create volumes for books root and config
VOLUME /books_root
VOLUME /config

# Run the container as our previously created non-root user.
USER 65534:65534

# Expose port 80 for the application.
EXPOSE 80

ENTRYPOINT ["/books", "--config", "/config"]
CMD ["serve", "-b", ":80"]
