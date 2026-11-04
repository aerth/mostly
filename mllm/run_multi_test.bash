# for testing both ollama and llama.cpp
# model

export DEBUG_LLM=2

pattern='.*'
if [ -n "$1" ]; then
  pattern="$1"
fi
make || exit 1
test1(){
echo "OLLAMA"
export LLM_HOST='https://ollama.home.arpa'
export LLM_MODEL=gemma4:26b
bin/mllm.test -test.v -test.failfast -test.run "$pattern" || exit 2
echo "OLLAMA PASS"
}
test2(){
echo "LLAMA.CPP"
export LLM_MODEL=gemma-4-E2B-it-Q4_K_M 
export LLM_HOST=http://127.0.0.1:9931/v1/chat/completions
bin/mllm.test -test.v -test.failfast -test.run "$pattern" || exit 3
echo "LLAMA.CPP PASS"
}

test2 && test1 && echo PASS
