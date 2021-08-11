#!/bin/bash

if [ $# -lt 2 ]; then
    >&2 echo "Usage: $0 PROJECT DB_PASSWORD"
    exit 1
fi

PROJECT=$1
DB_PASSWORD=$2

SCRIPT_DIR=$(cd $(dirname $0); pwd)

cd $SCRIPT_DIR || exit 1

docker build -t gcr.io/$PROJECT/game-logging-server . || exit 1

docker push gcr.io/$PROJECT/game-logging-server || exit 1

cat <<EOS | tr '\n' ',' | sed -e 's/,$//' > env.txt
DB_USER=root
DB_PASSWORD=$DB_PASSWORD
DB_HOST=/cloudsql/$PROJECT:asia-northeast1:game-logging-server
DB_NAME=game_logging_server
MIGRATIONS_DIR=/work/migrations
HOST=0.0.0.0
EOS

gcloud run deploy \
    game-logging-server \
    --image=gcr.io/$PROJECT/game-logging-server:latest \
    --allow-unauthenticated \
    --set-env-vars=`cat env.txt` \
    --set-cloudsql-instances=$PROJECT:asia-northeast1:game-logging-server \
    --platform=managed \
    --region=asia-northeast1 \
    --project=$PROJECT \
    || exit 1

rm env.txt || exit 1
