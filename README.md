# ffc
ffc is a wrapper around [ffmpeg](https://ffmpeg.org/) to make cutting videos to smaller chunks easier.

It's designed to cut one file at a time, but it is able to cut them to multiple pieces at once.

`ffc` uses copying, therefore time might be somewhat longer as ffmpeg will try to copy the content and not re-encode, therefore it will start at the first previous key frame instead of the exact frame at the given time.

## Usage

### Simply cut a video providing a simplified time form

In this mode time is expressed in a simple form, without using colons (`:`)

Cut `myvid.mpg` from 1 minute 23 to 2 minutes 4:

```
ffc myvid.mpg 123-204
```

This will result in a new file called `myvid-1ffc.mpg` that runs for roughly 37 seconds.

### Cut a video providing the desired seconds

Cut `myvid.mpg` from 1 minute 23 to 2 minutes 4.

```
ffc -s myvid.mpg 83-124
```

This will result in a new file called `myvid-1ffc.mpg` that runs for roughly 37 seconds.

### Cut a video into multiple pieces

Cut `myvid.mpg` from 1 minute 23 to 2 minutes 4, from 2 minutes 4 to 2 minute 52 and from 3 minute 8 to 4 minutes 12.

```
ffc myvid.mpg 123-204-252,308-412
```

This will result in three new files:
- `myvid-1ffc.mpg` that runs for roughly 37 seconds from roughly 1:23 of the original video.
- `myvid-2ffc.mpg` that runs for roughly 48 seconds from roughly 2:04 of the original video.
- `myvid-3ffc.mpg` that runs for roughly 64 seconds from roughly 3:08 of the original video.

### Cut a video into multiple pieces, append text

Cut `myvid.mpg` from 1 minute 23 to 2 minutes 4, from 2 minutes 4 to 2 minute 52 and from 3 minute 8 to 4 minutes 12, append each parts with "cut" instead of "ffc"

```
ffc myvid.mpg 123-204-252,308-412,402-412 256-303 cut
```

This will result in five new files:
- `myvid-1cut.mpg` that runs for roughly 37 seconds from roughly 1:23 of the original video.
- `myvid-2cut.mpg` that runs for roughly 48 seconds from roughly 2:04 of the original video.
- `myvid-3cut.mpg` that runs for roughly 64 seconds from roughly 3:08 of the original video.
- `myvid-4cut.mpg` that runs for roughly 10 seconds from roughly 4:02 of the original video.
- `myvid-5cut.mpg` that runs for roughly 7 seconds from roughly 2:56 of the original video.

### Cut a video into multiple pieces, advanced

Cut a 5 minutes long video at every 30 seconds, but ensure that pieces start 2 seconds earlier and end 3 seconds later

```
ffc -s -a 3 -b 2 myvid.mpg -30-60-90-120-150-180-210-240-270- cut
```

This will result in ten new files:
- `myvid-1cut.mpg` that runs for roughly 35 seconds from roughly 0:28 of the original video.
- `myvid-2cut.mpg` that runs for roughly 35 seconds from roughly 0:58 of the original video.
- `myvid-3cut.mpg` that runs for roughly 35 seconds from roughly 1:28 of the original video.
- `myvid-4cut.mpg` that runs for roughly 35 seconds from roughly 1:58 of the original video.
- `myvid-5cut.mpg` that runs for roughly 35 seconds from roughly 2:28 of the original video.
- `myvid-6cut.mpg` that runs for roughly 35 seconds from roughly 2:58 of the original video.
- `myvid-7cut.mpg` that runs for roughly 35 seconds from roughly 3:28 of the original video.
- `myvid-8cut.mpg` that runs for roughly 35 seconds from roughly 3:58 of the original video.
- `myvid-9cut.mpg` that runs for roughly 35 seconds from roughly 4:28 of the original video.
- `myvid-10cut.mpg` that runs for roughly 35 seconds from roughly 4:58 of the original video.

### Fix a given generated file

Fix `myvid-3cut.mpg` generated in the previous example to start earlier and end later

```
ffc -s -c 5 -o 3 myvid.mpg -30-60-90-120-150-180-210-240-270- cut
```

This will result in one new file:
- `myvid-3cut.mpg` that runs for roughly 40 seconds from roughly 1:25 of the original video.

## Additional tools

- [ffr](https://github.com/peteraba/ffr) is a toolbox which helps with cleaning up after extensive ffr usage. `ffr` is able to re-encode videos, merge generated names, prepend and append to filenames. `ffr` is designed to work with multiple files at once.
