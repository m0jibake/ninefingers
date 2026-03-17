# Ninefingers

Ninefingers is a cli tool for this simple two-step procedure: 
- download YouTube captions for provded video URL
- pass captions to NVIDIA provided LLM to obtain a summary


## How to use

1. Obtain API Key from NVIDIA
2. create .env and set NVIDIA_API_KEY
3. Execute
```
./ninefingers "https://www.youtube.com/watch?v=Q6nem-F8AG8&pp=ugUEEgJlbg%3D%3D" -v --model "moonshotai/kimi-k2-instruct"
```
