# Shared settings for up.sh, wait.sh and down.sh. Every value can be overridden by env.
PROJECT=${PROJECT:-go-bench-15205}
REGION=${REGION:-us-central1}
ZONE=${ZONE:-us-central1-a}
BUCKET=${BUCKET:-$PROJECT-results}
SUT_VM=${SUT_VM:-bench-sut}
LOADGEN_VM=${LOADGEN_VM:-bench-loadgen}
SUT_MACHINE=${SUT_MACHINE:-c2d-highcpu-4}
LOADGEN_MACHINE=${LOADGEN_MACHINE:-c2d-highcpu-16}
