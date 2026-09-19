-- infra/ratelimit/leakybucket.lua

-- KEYS[1] = bucket key (e.g., "ratelimit:user:uuid-123")
-- ARGV[1] = capacity (max water level)
-- ARGV[2] = rate (leak rate, requests per second)
-- ARGV[3] = now (current time in nanoseconds)
-- ARGV[4] = ttl (key TTL in seconds)
--
-- Returns: {allowed (0/1), remaining, retryAfterMs}

local key      = KEYS[1]
local capacity = tonumber(ARGV[1])
local rate     = tonumber(ARGV[2])
local now      = tonumber(ARGV[3])
local ttl      = tonumber(ARGV[4])

-- Читаем текущее состояние ведра
local bucket   = redis.call("HMGET", key, "water", "last_leak")
local water    = tonumber(bucket[1])
local lastLeak = tonumber(bucket[2])

-- Первое обращение — инициализируем пустое ведро
if water == nil then
    water = 0
    lastLeak = now
end

-- Вычисляем, сколько воды вытекло с последнего запроса
local elapsed = (now - lastLeak) / 1e9  -- наносекунды → секунды
local leaked  = elapsed * rate
water = math.max(0, water - leaked)

-- Проверяем, поместится ли новый запрос
if water + 1 > capacity then
    local overflow  = (water + 1) - capacity
    local waitSec   = overflow / rate
    local retryMs   = math.ceil(waitSec * 1000)
    local remaining = math.max(0, math.floor(capacity - water))

    redis.call("HSET", key, "water", water, "last_leak", now)
    redis.call("EXPIRE", key, ttl)

    return {0, remaining, retryMs}
end

-- Запрос разрешён — добавляем воду
water = water + 1
local remaining = math.max(0, math.floor(capacity - water))

redis.call("HSET", key, "water", water, "last_leak", now)
redis.call("EXPIRE", key, ttl)

return {1, remaining, 0}