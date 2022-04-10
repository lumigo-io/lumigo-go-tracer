resource "aws_lambda_function" "otel" {
  function_name = "OpenTelemetryHandler"

  s3_bucket = aws_s3_bucket.lambda_bucket.id
  s3_key    = aws_s3_bucket_object.lambda_otel.key

  runtime = "go1.x"
  handler = "otel"

  source_code_hash = data.archive_file.lambda_otel.output_base64sha256

  role = aws_iam_role.lambda_exec.arn

  layers = ["arn:aws:lambda:us-east-1:114300393969:layer:lumigo-tracer-extension:37"]

   environment {
    variables = {
      LUMIGO_DEBUG = "true"
      LUMIGO_USE_TRACER_EXTENSION = "true"
    }
  }
}

resource "aws_cloudwatch_log_group" "otel" {
  name = "/aws/lambda/${aws_lambda_function.otel.function_name}"

  retention_in_days = 30
}

resource "aws_iam_role" "lambda_exec" {
  name = "serverless_lambda"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = "sts:AssumeRole"
      Effect = "Allow"
      Sid    = ""
      Principal = {
        Service = "lambda.amazonaws.com"
      }
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "lambda_policy" {
  role       = aws_iam_role.lambda_exec.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_iam_role_policy" "lambda_s3_read" {
  name = "lambda_s3_read_policy"
  role = aws_iam_role.lambda_exec.id

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
            "s3:*"
        ],
      "Effect": "Allow",
      "Resource": "*"
    },
    {
      "Action": [
            "ssm:*"
        ],
      "Effect": "Allow",
      "Resource": "*"
    }
  ]
}
EOF
}