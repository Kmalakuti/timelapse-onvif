FROM python:3.12-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg ca-certificates fonts-dejavu-core \
    libjpeg62-turbo libpng16-16 libfreetype6 \
  && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY app ./app
COPY templates ./templates
COPY tests ./app/tests
COPY static ./static
COPY worker.proto ./worker.proto

# Generate gRPC python stubs
RUN python -m grpc_tools.protoc -I. --python_out=app --grpc_python_out=app worker.proto

CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8080"]
