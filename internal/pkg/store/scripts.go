package store

// AtomicDeductStock Lua 脚本
// 逻辑：
// 1. 检查 Key 是否存在 (是否预热过) -> 返回 -1
// 2. 检查库存是否足够 -> 返回 0
// 3. 够就扣减 -> 返回 1
const AtomicDeductStock = `
local key = KEYS[1]
local amount = tonumber(ARGV[1])
local current = redis.call('get', key)

if current == false then
    return -1
end

if tonumber(current) >= amount then
    redis.call('decrby', key, amount)
    return 1
else
    return 0
end
`
