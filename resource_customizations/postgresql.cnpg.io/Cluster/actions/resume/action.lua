if obj.metadata == nil then
    obj.metadata = {}
end

if obj.metadata.annotations == nil then
    obj.metadata.annotations = {}
end

obj.metadata.annotations["cnpg.io/reconciliation"] = "disabled"
return obj
