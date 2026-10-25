cmake_minimum_required(VERSION 3.20)
project(msgfmt)

set(CMAKE_CXX_STANDARD 17)

add_compile_options(-Wall -Wextra)

include_directories(include)

add_executable(msgfmt
        src/main.cpp
        src/msgfmt.cpp
)
