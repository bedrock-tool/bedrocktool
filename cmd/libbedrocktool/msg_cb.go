package main

/*
#include <stdint.h>
typedef void (*bedrocktoolHandler)(uint8_t* data, size_t length);
void call_bedrocktool_handler(bedrocktoolHandler h, uint8_t* data, size_t length) {
	h(data, length);
}
*/
import "C"
