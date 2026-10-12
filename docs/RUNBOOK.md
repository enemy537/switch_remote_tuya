# Runbook

How to deploy Switch Remote from nothing, and how to operate it afterwards. Commands run from the repository root unless a step says otherwise.

> This runbook was written alongside the code and has not yet been executed end to end against a real AWS account, Neon database or Tuya project. Treat the first deployment as its test and correct any step that differs.

## 1. Prerequisites

Accounts:

- AWS account with permission to create S3, CloudFront, Lambda, IAM and CloudWatch Logs resources.
- Neon account (free tier is enough).
- Tuya developer account at <https://platform.tuya.com>.

Tools on the machine you deploy from:

| Tool | Version | Used for |
|---|---|---|
| Go | 1.25 or newer | Building the Lambda |
| Node.js | 20.19 or newer | Building the UI |
| Terraform | 1.6 or newer | Creating AWS resources |
| AWS CLI | v2, with credentials configured | Uploading the UI, reading logs |
| `psql` | any recent | Applying the database schema |
| `zip`, `make` | any | Packaging the Lambda |

On the original development machine, Go 1.25 is installed side by side as `~/go/bin/go1.25.0`, and Node 20 is selected with `nvm use 20`. Terraform is not installed there.

## 2. Tuya cloud project (once)

1. Sign in at <https://platform.tuya.com> and open **Cloud > Development > Create Cloud Project**.
2. Choose industry **Smart Home**, development method **Smart Home**, data center **Western America**.
3. When asked which API services to authorize, keep the defaults and make sure **IoT Core** and **Smart Home Basic Service** are included.
4. Open the project's **Overview** tab and copy the **Access ID** (client ID) and **Access Secret** (client secret).

The data center must match the region the Smart Life accounts are registered in. If your users are outside the Americas, pick the matching data center and set `tuya_base_url` in step 5 (for example `https://openapi.tuyaeu.com` for Central Europe).

The IoT Core service on the free plan is a time-limited trial that has to be extended from **Cloud > Cloud Services**. When it lapses, every Tuya call fails; see Troubleshooting.

## 3. Neon database (once)

1. Create a Neon project in an AWS region near `us-west-2`.
2. Copy the **pooled** connection string. Its host contains `-pooler`, and it ends with `?sslmode=require`.
3. Apply the schema:

   ```bash
   psql "<connection string>" -f back/migrations/001_init.sql
   ```

4. Create the first admin. Use a lowercase email and a password of at least 8 characters:

   ```bash
   psql "<connection string>" -c "insert into users (email, password_hash, is_admin) values ('you@example.com', crypt('YOUR-PASSWORD', gen_salt('bf', 10)), true);"
   ```

   This leaves the password in your shell history. Change it from the admin page after the first login, or clear the history entry.

## 4. Build the API

```bash
make -C back build        # on the original machine: make -C back GO=$HOME/go/bin/go1.25.0 build
```

This writes `back/build/api.zip`, which Terraform uploads in the next step.

## 5. Create the AWS resources

```bash
cd infra
cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars`:

| Variable | Value |
|---|---|
| `database_url` | The pooled Neon connection string from step 3 |
| `tuya_client_id` | Access ID from step 2 |
| `tuya_client_secret` | Access Secret from step 2 |
| `jwt_secret` | Output of `openssl rand -hex 32` |
| `region` | Optional, defaults to `us-west-2` |
| `tuya_base_url` | Optional, defaults to `https://openapi.tuyaus.com` |

Then:

```bash
terraform init
terraform apply
```

CloudFront takes several minutes to create. Note the outputs: `app_url`, `site_bucket`, `distribution_id`, `function_name`.

Terraform state is kept locally in `infra/terraform.tfstate` and contains the secrets in plain text. It is git-ignored; back it up somewhere private, because losing it means Terraform no longer knows about the resources.

## 6. Deploy the UI

```bash
cd front
npm ci
npm run build
aws s3 sync dist "s3://$(terraform -chdir=../infra output -raw site_bucket)" --delete
aws cloudfront create-invalidation \
  --distribution-id "$(terraform -chdir=../infra output -raw distribution_id)" --paths '/*'
```

## 7. Check the deployment

1. Open `app_url` in a browser. The login page should appear.
2. Log in as the admin created in step 3.
3. Open **Admin**. The user list should show your account.

If the login request fails, read the Lambda logs (see Operations).

## 8. Onboard a user

Repeat for each user.

1. The user installs **Smart Life**, creates an account in the same region as the Tuya data center, and pairs their switch. The Mini Tuya Wi-Fi 16A switch only joins 2.4 GHz Wi-Fi.
2. In the Tuya console, open the project, then **Devices > Link App Account > Add App Account**. A QR code appears.
3. The user scans it from Smart Life (**Me**, then the scan icon at the top right) and confirms. The QR code expires quickly, so do this together, for example over a screen share.
4. The account now appears in the linked accounts list with its **UID**. Copy it.
5. In the app, open **Admin > Create user** and enter the user's email, a password and that UID.
6. The user logs in, clicks **Add device**, picks the switch and registers it.

A Smart Life account can be linked to at most two Tuya projects.

## Operations

### Read the API logs

```bash
aws logs tail /aws/lambda/switch-remote-api --follow
```

Logs are kept for 14 days.

### Deploy a new API version

```bash
make -C back build
terraform -chdir=infra apply
```

### Deploy a new UI version

Repeat step 6.

### Change a secret

Edit `infra/terraform.tfvars` and run `terraform -chdir=infra apply`. Changing `jwt_secret` logs every user out.

### Reset a user's password

**Admin > Users > Reset password**. If the only admin is locked out, set it in the database:

```bash
psql "<connection string>" -c "update users set password_hash = crypt('NEW-PASSWORD', gen_salt('bf', 10)) where email = 'you@example.com';"
```

### Remove a user

There is no screen for this. Deleting the row also deletes that user's device registrations; the switches stay in their Smart Life account.

```bash
psql "<connection string>" -c "delete from users where email = 'user@example.com';"
```

### Run locally

```bash
# terminal 1
export DATABASE_URL=... TUYA_CLIENT_ID=... TUYA_CLIENT_SECRET=... JWT_SECRET=dev-secret
make -C back run          # API on http://localhost:8080

# terminal 2
cd front && npm install && npm run dev    # UI with /api proxied to :8080
```

The session cookie is marked Secure. Chrome and Firefox still accept it on `localhost`; Safari does not.

### Tear everything down

```bash
terraform -chdir=infra destroy
```

This deletes the AWS resources, including the UI bucket and its files. The Neon database and the Tuya project are separate and must be deleted in their own consoles.

## Troubleshooting

When a Tuya call fails, the UI only shows "could not reach Tuya, try again". The Tuya error code and message are in the API logs, on a line starting with `tuya`.

| Symptom | Likely cause | Fix |
|---|---|---|
| "Add device" shows no switches | The Smart Life account is not linked to the project, the UID on the user is wrong, or the switch is already registered | Check **Devices > Link App Account** in the Tuya console and compare the UID with the one in **Admin > Users** |
| A user sees no switches and no error | The user was created with an empty Tuya UID | There is no screen to edit it; set it with `update users set tuya_uid = '...' where email = '...'` |
| Tuya error `1106 permission deny` | The device belongs to an account that is not linked to this project | Link the account (step 8) |
| Tuya error `1004 sign invalid` | Wrong client secret, or the data center does not match `tuya_base_url` | Fix `terraform.tfvars` and apply |
| Tuya error `28841002` or a message about an expired subscription | The IoT Core trial has lapsed | Extend it under **Cloud > Cloud Services** in the Tuya console |
| A switch shows offline | It has no power or lost Wi-Fi | Check it in Smart Life; the app cannot control an offline switch |
| Toggling succeeds but nothing happens | The switch uses a command code other than `switch_1` | In the Tuya console open the device's **Debug Device** page, find the on/off code, and change the constant in `back/internal/tuya` |
| The UI still shows the old version after a deploy | CloudFront cache | Run the invalidation from step 6 |
| Every API call returns 403 with an AWS error body | The Function URL permissions are missing | Run `terraform -chdir=infra apply` again and check both `aws_lambda_permission` resources exist |
| Every API call returns 500 or times out | The Lambda cannot reach Neon | Check `database_url`, and that the Neon project is not suspended or deleted |
