Set up a vault development server:

    `vault server -dev`
    `export VAULT_DEV_ROOT_TOKEN_ID="..."
    ensure vault binary in /usr/local/bin and attached to the PATH in .bashrc `cat ~/.bashrc`
    `export VAULT_ADDR="http://127.0.0.1:8200"
    `vault status` to ensure vault server running
    `vault secrets enable aws`
    `vault write aws/config/root access_key="..." secret_key="..." region="us-east-1"` with jnkroeker IAM user credentials
    `vault read aws/config/root` 
    `vault secrets list`
    `vault write aws/roles/assume_role_s3_sqs role_arns=<vault_s3_sqs_engineer ARN> credential_type=assumed_role`
    `vault write aws/sts/assume_role_s3_sqs ttl=15m`

Set up a redis development server:

    `./redis-server`