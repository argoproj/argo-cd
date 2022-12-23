local io = require("io")

local file = io.open("job-manifest-template.yaml", "r");
assert(file);
local data = file:read("*a"); -- Read everything
file:close();

obj.metadata.annotations["kuku"] = "muku"
return obj
