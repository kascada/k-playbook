ARG PYTHON_TAG=3.12-slim
FROM python:${PYTHON_TAG} AS builder
RUN pip install poetry==1.8.2
FROM builder AS runtime
FROM redis:7.2.4@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
FROM ghcr.io/example/base:${UNKNOWN_TAG}
