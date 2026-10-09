#ifndef CXXPROBE_H
#define CXXPROBE_H

#ifdef __cplusplus
extern "C" {
#endif

// Returns a malloc'd report of what the probe saw; the caller frees it.
char* cxxprobe_run(const char* input);

#ifdef __cplusplus
}
#endif

#endif  // CXXPROBE_H
