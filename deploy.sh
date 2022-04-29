#!/bin/bash

if [ $# -lt 1 ]; then
    >&2 echo "Usage: $0 PROJECT"
    exit 1
fi

PROJECT=$1

SCRIPT_DIR=$(cd $(dirname $0); pwd)

cd $SCRIPT_DIR || exit 1

docker build -t gcr.io/$PROJECT/game-logging-server . || exit 1

docker push gcr.io/$PROJECT/game-logging-server || exit 1

cat <<EOS | tr '\n' ',' | sed -e 's/,$//' > env.txt
BUCKET=tsujio-game-log
HOST=0.0.0.0
EOS

gcloud run deploy \
    game-logging-server \
    --image=gcr.io/$PROJECT/game-logging-server:latest \
    --allow-unauthenticated \
    --set-env-vars=`cat env.txt` \
    --platform=managed \
    --region=asia-northeast1 \
    --project=$PROJECT \
    || exit 1

rm env.txt || exit 1
