#!/usr/bin/env bash
# One-time bootstrap: creates a GCS bucket for Terraform remote state.
#
# Usage:
#   ./bootstrap.sh <GCP_PROJECT_ID>
#
# After running:
#   terraform init -backend-config="bucket=<PROJECT_ID>-terraform-state"

set -euo pipefail

PROJECT_ID="${1:?Usage: ./bootstrap.sh <GCP_PROJECT_ID>}"
BUCKET_NAME="${PROJECT_ID}-terraform-state"
REGION="australia-southeast1"

echo "=> Enabling storage API..."
gcloud services enable storage.googleapis.com --project="${PROJECT_ID}"

echo "=> Creating GCS bucket gs://${BUCKET_NAME} in ${REGION}..."
if gsutil ls -b "gs://${BUCKET_NAME}" &>/dev/null; then
  echo "   Bucket already exists, skipping."
else
  gsutil mb -p "${PROJECT_ID}" -l "${REGION}" -b on "gs://${BUCKET_NAME}"
fi

echo "=> Enabling versioning..."
gsutil versioning set on "gs://${BUCKET_NAME}"

echo ""
echo "Done! Next steps:"
echo "  cd infrastructure"
echo "  cp terraform.tfvars.example terraform.tfvars  # fill in values"
echo "  terraform init -backend-config=\"bucket=${BUCKET_NAME}\""
echo "  terraform plan"
echo "  terraform apply"
