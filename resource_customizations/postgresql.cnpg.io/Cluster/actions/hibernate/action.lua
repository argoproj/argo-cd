-- Enter CloudNativePG declarative hibernation by setting the hibernation annotation to "on".
-- https://cloudnative-pg.io/docs/current/declarative_hibernation/
if obj.metadata == nil then
    obj.metadata = {}
end

if obj.metadata.annotations == nil then
    obj.metadata.annotations = {}
end

obj.metadata.annotations["cnpg.io/hibernation"] = "on"
return obj
