cmake_minimum_required(VERSION 3.20)

set(CMAKE_CXX_STANDARD 17)

add_compile_options(-Wall -Wextra)

include_directories(include)

add_executable(msgfmt
    src/main.cpp
    src/msgfmt.cpp
)

enable_testing()

add_executable(msgfmt_tests
    tests/test_msgfmt.cpp
    src/msgfmt.cpp
)

add_test(NAME msgfmt_tests COMMAND msgfmt_tests)
