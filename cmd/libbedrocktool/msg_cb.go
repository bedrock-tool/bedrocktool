package main

/*
typedef char* (*msg_cb)(const char* message);
char* bridge_msg_cb(const char* message, msg_cb cb){
    return cb(message);
}
*/
import "C"
