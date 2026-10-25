#include "msgfmt.h"
#include <iostream>
#include <string>

int main(int argc, char* argv[]) {
    std::string input;
    char c;
    while (std::cin.get(c)) {
        input.push_back(c);
    }

    if (!msgfmt::is_valid_utf8(input)) {
        std::cout << "{\"error\": \"invalid_utf8\"}\n";
        return 2;
    }

    if (argc == 2 && std::string(argv[1]) == "escape") {
        std::cout << msgfmt::escape(input);
        return 0;
    }

    return 0;
}
