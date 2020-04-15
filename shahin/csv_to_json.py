import csv
import json
import sys

in_file = sys.argv[1]
out_file = sys.argv[2]

lights = {}
with open(in_file, 'r') as f:
	rd = csv.reader(f)
	for row in rd:
		if len(row) != 5 or '.jpg' not in row[4]:
			continue
		lights[row[4]] = json.loads(row[2])

with open(out_file, 'w') as f:
	json.dump(lights, f)
