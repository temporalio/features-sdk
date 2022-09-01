# Build in a full featured container
FROM golang:1.19 as build

WORKDIR /app

# Copy CLI build dependencies
COPY features ./features
COPY harness ./harness
COPY cmd ./cmd
COPY go.mod go.sum main.go ./

# Build the CLI
RUN CGO_ENABLED=0 go build

ARG SDK_VERSION
ARG SDK_REPO_URL
ARG SDK_REPO_REF
# Could be a cloned lang SDK git repo or just an arbitrary file so the COPY command below doesn't fail.
# It was either this or turn the Dockerfile into a template, this seemed simpler although a bit awkward.
ARG REPO_DIR_OR_PLACEHOLDER
COPY ./${REPO_DIR_OR_PLACEHOLDER} ./${REPO_DIR_OR_PLACEHOLDER}

# Prepare the feature for running
RUN ./sdk-features prepare --lang go --dir prepared --version "$SDK_VERSION"

# Copy the CLI and prepared feature to a distroless "run" container
FROM gcr.io/distroless/static-debian11:nonroot

COPY --from=build /app/sdk-features /app/sdk-features
COPY --from=build /app/features /app/features
COPY --from=build /app/prepared /app/prepared
CMD ["/app/sdk-features", "run", "--lang", "go", "--prepared-dir", "prepared"]