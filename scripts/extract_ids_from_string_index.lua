-- Copyright 2015 Alex Browne.  All rights reserved.
-- Use of this source code is governed by the MIT
-- license, which can be found in the LICENSE file.

-- exctract_ids_from_string_index is a lua script that takes the following arguments:
-- 	1) setKey: The key of a sorted set for a string index, where each member is of the
--			form: value + NULL + id, where NULL is the ASCII NULL character which has a codepoint
--			value of 0.
--		2) destKey: The key of a sorted set where the resulting ids will be stored
-- 	3) min: The min argument for the ZRANGEBYLEX command
-- 	4) max: The end argument for the ZRANGEBYLEX command
-- The script then extracts the ids from setKey using the given min and max arguments,
-- and then stores them destKey with the appropriate scores in ascending order.

-- Assign keys to variables for easy access
local setKey = KEYS[1]
local destKey = KEYS[2]
local min = ARGV[1]
local max = ARGV[2]
-- Get all the members (value+id pairs) from the sorted set
local members = redis.call('ZRANGEBYLEX', setKey, min, max)
if #members > 0 then
	-- Iterate over the members and extract the ids
	for i, member in ipairs(members) do
		-- The id is everything after the last space
		-- Find the index of the last space
		local idStart = string.find(member, '%z[^%z]*$')
		local id = string.sub(member, idStart+1)
		redis.call('ZADD', destKey, i, id)
	end
end