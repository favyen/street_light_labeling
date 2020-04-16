from discoverlib import geom
import json
import os
import random
import subprocess

SIZE = 512

with open('labels.json', 'r') as f:
	labels = json.load(f)

counter = 0
examples = []
for fname, points in labels.items():
	points = [geom.Point(p[0], p[1]) for p in points]
	objects = [p.bounds().add_tol(50) for p in points]
	rect = geom.Rectangle(
		geom.Point(0, 0),
		geom.Point(SIZE, SIZE)
	)
	objects = [rect.clip_rect(obj_rect) for obj_rect in objects]
	crop_objects = []
	for obj_rect in objects:
		start = geom.FPoint(float(obj_rect.start.x) / SIZE, float(obj_rect.start.y) / SIZE)
		end = geom.FPoint(float(obj_rect.end.x) / SIZE, float(obj_rect.end.y) / SIZE)
		crop_objects.append((start.add(end).scale(0.5), end.sub(start)))

	example_path = '/mnt/signify/la/yolo-shahin/images/{}.jpg'.format(counter)
	subprocess.call(['cp', '/mnt/signify/la/shahin-dataset/training_v1/' + fname, example_path])
	crop_lines = ['0 {} {} {} {}'.format(center.x, center.y, size.x, size.y) for center, size in crop_objects]
	with open('/mnt/signify/la/yolo-shahin/images/{}.txt'.format(counter), 'w') as f:
		f.write("\n".join(crop_lines) + "\n")

	examples.append(example_path)
	counter += 1

random.shuffle(examples)
val_set = examples[0:32]
train_set = examples[32:]

with open('/mnt/signify/la/yolo-shahin/train.txt', 'w') as f:
	f.write("\n".join(train_set) + "\n")
with open('/mnt/signify/la/yolo-shahin/test.txt', 'w') as f:
	f.write("\n".join(val_set) + "\n")
with open('/mnt/signify/la/yolo-shahin/valid.txt', 'w') as f:
	f.write("\n".join(val_set) + "\n")
