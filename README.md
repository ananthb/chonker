# RANGER

*Download large files in parallel chunks in Go.*

### Why? 

Current Go HTTP clients download files as a stream of bytes, usually with a buffer. If you consider every file an array of bytes, this means that when you intitiate a download a connection is opened and you receive an `io.Reader`. As you `Read` bytes off this `Reader`, more bytes are loaded up into an internal buffer (an in-memory byte array that stores a certain amount of data in the expectation that you'll read it soon). As you keep reading data, the HTTP client will fill the buffer up as fast as it can from the server. 

### So? Why is this a problem? 

Most of the time this is what we want and need. When we're downloading large files (say from Amazon S3 or CloudFront, or any other fileserver) this is usually not optimal. These services have per-connection speed limits on the bytes going out, and if you're donwloading a very large file (over 25 GB) you're also not likely to be using the caches. This means that the number of bytes coming in per second is usually lower than what your connection actually supports. 

### What does Ranger do? 

Ranger does two orthogonal things to speed up transfers — one, it downloads files in chunks: so if there are 1000 bytes, for example, it can download the file in chunks of 100 bytes, by requesting byte range 0-99, 100-199, 200-299 and so on using a HTTP RANGE GET. This allows the service to cache each chunk, because even if the total file size is too large to cache each chunk is small enough to fit. See the [CloudFront Developer Guide](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/RangeGETs.html) for more information.

Two, it downloads upcoming chunks in parallel, so if the parallelism count is set at 3, in the example above it would download byte ranges 0-99, 100-199 and 200-299 in parallel, even while the first range is being `Read`. It will also start downloading the fourth range after the first one is read, and so on. This allows trading RAM for speed - agreeing to dedicate 3 x 100 bytes of memory allows downloads to go on that much faster. In practice, 16MB is a good chunk size, especially if that lines up with the multipart upload configuration in a system like S3. See the [S3 Developer Guide](https://docs.aws.amazon.com/whitepapers/latest/s3-optimizing-performance-best-practices/use-byte-range-fetches.html) for more information. 








