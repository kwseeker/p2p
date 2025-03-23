cd ../
docker build --tag signal-server:0.0.1 .
docker build -f Dockerfile.answer --tag peer_answer:0.0.1 .
docker build -f Dockerfile.offer --tag peer_offer:0.0.1 .