import math
import sys
import yaml

def represent_str(dumper, data):
    use_literal_style = '\n' in data or ' ' in data or '\\' in data
    if use_literal_style:
        data = data.replace("\\", "\\\\")
        return dumper.represent_scalar("tag:yaml.org,2002:str", data, style="|")
    return dumper.represent_str(data)

yaml.add_representer(str, represent_str, Dumper=yaml.SafeDumper)

# Technically we can write to the same file, but that can erase the file if dump fails.
input_filename = sys.argv[1]
output_filename = sys.argv[2]

with open(input_filename, "r") as input_file:
    data = yaml.safe_load_all(input_file.read())

with open(output_filename, "w") as output_file:
    # Without large width the multiline strings enclosed in "..." are rendered weirdly.
    yaml.safe_dump_all(data, output_file, width=math.inf)
