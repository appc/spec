package device

/*
#define _BSD_SOURCE
#define _DEFAULT_SOURCE
#include <sys/types.h>

unsigned int
my_major(dev_t dev)
{
  return major(dev);
}

unsigned int
my_minor(dev_t dev)
{
  return minor(dev);
}
*/
import "C"

func Major(rdev uint64) uint {
	major := C.my_major(C.dev_t(rdev))
	return uint(major)
}

func Minor(rdev uint64) uint {
	minor := C.my_minor(C.dev_t(rdev))
	return uint(minor)
}
