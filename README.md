Set up a vault development server:

    `vault server -dev`
    x `export VAULT_DEV_ROOT_TOKEN_ID="..." 
    put  `Root Token` in .env as `CH_VAULT_STATIC_TKN`
    ensure vault binary in /usr/local/bin and attached to the PATH in .bashrc `cat ~/.bashrc`
    `export VAULT_ADDR="http://127.0.0.1:8200" so we can run cmd line
    put the vault address in .env as `CH_VAULT_ADDR`
    `vault status` to ensure vault server running
    `vault secrets enable aws`

    `vault write aws/config/root 
    access_key="..." secret_key="..." region="us-east-1"` 
    with jnkroeker IAM user credentials 

    `vault read aws/config/root` 
    `vault secrets list`

    `vault write aws/roles/assume_role_s3_sqs 
    role_arns=<vault_s3_sqs_engineer ARN> credential_type=assumed_role`

    //dont do this if you want to run chyme right away
    `vault write aws/sts/assume_role_s3_sqs ttl=15m`

Set up a redis development server:

    from /usr/bin run `./redis-server`

Set Docker Host environment variable and username in config:

    x `export DOCKER_HOST="tcp://0.0.0.0:2375"`
    export DOCKER_HOST="unix:///var/run/docker.sock"

    CH_WORKER_DOCKER_USER='john'
    CH_WORKER_WORKDIR='/home/john/'

Run the CLI:

    x first update vault static token in /cmd/util.go
    x `go build`
    x `go install chyme`

    cd ~/go/src/chyme
    `make build`

Currently supported commands (* = required):

    * `./out/chyme help`

    --- Open new terminal window for following, remaining commands ---

    `./out/chyme indexer start`
 
    `./out/chyme indexer ingest s3://{*BUCKET}/{KEY} --filter 'ext/{FILE_TYPE}' --recursion {DEPTH}` 

    open redis server with `redis-cli` command
    `KEYS *`
    `SMEMBERS "<key name>"` because the value type at the the key is a SET

    --- After ingesting to Redis, start tasker to add tasks to SQS queue ---

    `./out/chyme tasker start`

    --- After tasker adds tasks to SQS queue, start worker ---

    `./out/chyme worker start`

Kill redis-server :

    open redis-cli

    `shutdown NOSAVE`

CHANGELOG: 

    2020-07-26: s3 url scheme now `s3://<bucket-name>/<key-name>`

    2020-08-16: packaged as a go module now, to run execute `./out/chyme <command>`

                * `./out/chyme indexer ingest s3://june-test-bucket-jnk/ --filter 'ext/jpg' --recursion 3` succeeds in injesting all jpg in the bucket

    2020-08-23: there cant be two commands started at the same time with `start`
                two commands:
                    `./out/chyme indexer start`
                    `./out/chyme indexer ingest s3://...`
                    `./out/chyme tasker start`
                
                ! kill indexer before starting tasker

    2020-08-24: Upgraded redis-server and redis-cli from 3.0 to 6.0.6
                using: `https://medium.com/@toniflorithomar/upgrade-redis-82f28b06a8eb`
                    SPopN was erroring out with size parameter
                        required by go-redis api but available only from redis ^3.2

    2021-04-22: chyme successfully creates tasks for .mov files in SQS queue

                * `./out/chyme indexer ingest s3://jnk-golf/ --filter 
                    'ext/mov' --recursion 3`
                
                * `./out/chyme tasker start`

                * `./out/chyme worker start`

    2021-05-06: worker successfully starts container with ffmpeg and bento4 installed
                from .mov template

                Dockerfile for image on local machine at ~/mov_converter

    2021-05-08: worker creates /input /output directories on local machine using CH_WORKER_WORKDIR
                    and downloads the .mov resource specified in the task to /input
    
                worker correctly loads the .mov resource from local <CH_WORKER_WORKDIR>/chyme/input
                    onto container at /in 
    
    2021-05-21: all stages of chyme work; 
                    pulls a .mov file from S3, processes it using ffmpeg docker container, uploads mpg-dash manifest file to another S3 bucket

