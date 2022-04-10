provider "aws" {
  region = var.aws_region
}

resource "random_pet" "lambda_bucket_name" {
  prefix = "sink"
  length = 4
}

resource "aws_s3_bucket" "lambda_bucket" {
  bucket = random_pet.lambda_bucket_name.id

  acl           = "private"
  force_destroy = true
}

data "archive_file" "lambda_otel" {
  type = "zip"

  source_dir  = "${path.module}/bin"
  output_path = "${path.module}/otel.zip"
}

resource "aws_s3_bucket_object" "lambda_otel" {
  bucket = aws_s3_bucket.lambda_bucket.id

  key    = "otel.zip"
  source = data.archive_file.lambda_otel.output_path

  etag = filemd5(data.archive_file.lambda_otel.output_path)
}