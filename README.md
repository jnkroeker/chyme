### Quick Start

1. Set up a vault development server:

    a. execute `vault server -dev`

        `brew services restart vault` can clean up and restart vault on Mac

        `top` to see running processes on Mac: `kill -9 <#>` to stop process

    b. put `Root Token` in .env as `CH_VAULT_STATIC_TKN`

    c. ensure vault binary in /usr/local/bin and attached to the PATH in .bashrc with:

        `echo $PATH | grep /usr/local/bin` or `cat ~/.bashrc` 

    d. execute `export VAULT_ADDR="http://127.0.0.1:8200"` so we can run cmd line

    e. put the vault address, http://localhost:8200 , in .env as `CH_VAULT_ADDR`

    f. execute `vault status` to ensure vault server running

    g. execute `vault secrets enable aws` 
        
        this allows us to store AWS secrets in Vault

    h. execute `vault write aws/config/root access_key="..." secret_key="..." region="us-east-1"` 
    
        fill in AWS IAM user credentials

    i. execute `vault read aws/config/root` 

    j. execute `vault secrets list`

    k. provide a role for the Chyme operation to assume that will give access to sqs and s3
    
    `vault write aws/roles/assume_role_s3_sqs role_arns=<vault_s3_sqs_engineer ARN> credential_type=assumed_role`
    
        this is the ARN for the role you created for this purpose: arn:aws:iam::966216697299:role/vault_s3_sqs_engineer

    // dont execute this if you want to run chyme right away
    l. assume the role by executing `vault write aws/sts/assume_role_s3_sqs ttl=15m`

2. Set up a redis development server:

    a. If running Chyme from Ubuntu, from /usr/bin execute `./redis-server`

       If running from Mac, redis is installed with homebrew, so execute `redis-server` from anywhere

    b. if following error encountered:
        Could not create server TCP listening socket *:6379: bind: Address already in use

        execute `sudo service redis-server stop` if on Ubuntu

3. Set Docker Host environment variable and username in config:

    a. execute `export DOCKER_HOST="unix:///var/run/docker.sock"`

    b. ensure the following set in .env:
        
        this is the working directory on the local machine.
        and the user is the user built into the image in the Dockerfile

        on Ubuntu desktop it is:

        CH_WORKER_DOCKER_USER='john'
        CH_WORKER_WORKDIR='/home/john/'

        on Mac laptop it is:

        CH_WORKER_DOCKER_USER='john'
        CH_WORKER_WORKDIR='/Users/johnkroeker/'

4. Build docker image for mov processing

    a. update CONVERTER_VERSION variable in Makefile

    b. update the image version number in /internal/tasker/template/mov.go to match CONVERTER_VERSION

    b. execute `make mov_converter`

    x execute `docker build . jnkroeker/mov_converter:<version>

5. Run the CLI:

    cd ~/go/src/chyme
    `make build`

### Currently supported commands (* = required):

    * `./out/chyme help`

*** Open new terminal window for following commands ***

#### Indexing S3 Bucket

    `./out/chyme indexer start`
 
    `./out/chyme indexer ingest s3://{*BUCKET}/{KEY} --filter 'ext/{FILE_TYPE}' --recursion {DEPTH}` 

        Chyme only has a processing template for .MOV files at present.

            --filter 'ext/mov'

        use test bucket s3://june-test-bucket-jnk/video/

    open redis server with `redis-cli` command

        `KEYS *`

        `SMEMBERS "<key name>"` because the value type at the the key is a SET

#### Add tasks for processing the indexed bucket in Redis

    --- After ingesting to Redis, start tasker to add tasks to SQS queue ---

    `./out/chyme tasker start`

#### Start processing the queued tasks

    --- After tasker adds tasks to SQS queue, start worker ---

    `./out/chyme worker start`

#### Clean up

    Kill redis-server :

        open redis-cli

        `shutdown NOSAVE`

### CHANGELOG: 

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
                    pulls a .mov file from S3, processes it using ffmpeg docker container, uploads mpg-dash manifest file to another S3

    2022-12-10: upgraded vault to v1.10.0

                need to add graceful shutdown of worker when interrupt signal received

    2023-01-04: added a go binary for metadata extraction (from my exorcist project) to the mov_converter Docker image

                added lines to images/mov/process_mov.sh to execute the extractor binary

    2023-01-05: force dec-2022 branch as new master; master would not execute tasks using worker

