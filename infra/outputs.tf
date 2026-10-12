output "app_url" {
  value = "https://${aws_cloudfront_distribution.app.domain_name}"
}

output "site_bucket" {
  value = aws_s3_bucket.site.id
}

output "distribution_id" {
  value = aws_cloudfront_distribution.app.id
}

output "function_name" {
  value = aws_lambda_function.api.function_name
}
