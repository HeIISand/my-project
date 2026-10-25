#include "msgfmt.h"
#include <iostream>
#include <string>
#include <vector>

using msgfmt::Range;
using msgfmt::ParsedResult;

void require(bool cond) {
    if (!cond) {
        std::exit(1);
    }
}

void check_parse(const std::string& input,
                 const std::string& expected_text,
                 const std::vector<Range>& expected_ranges) {
    ParsedResult res = msgfmt::parse(input);
    if (res.text != expected_text || res.ranges != expected_ranges) {
        std::cerr << "Failed for input: " << input << "\n";
        std::cerr << "Expected text: [" << expected_text << "], got: [" << res.text << "]\n";
        std::cerr << "Expected ranges count: " << expected_ranges.size()
                  << ", got: " << res.ranges.size() << "\n";
        for (size_t i = 0; i < res.ranges.size(); ++i) {
            std::cerr << "  got[" << i << "]: " << res.ranges[i].style
                      << "(" << res.ranges[i].offset << ", " << res.ranges[i].length
                      << ", " << res.ranges[i].url << ")\n";
        }
        std::exit(1);
    }
}

void test_appendix_a_examples() {
    check_parse("**важно**", "важно", {
        {"bold", 0, 5, ""}
    });

    check_parse("Привет, **мир**!", "Привет, мир!", {
        {"bold", 8, 3, ""}
    });

    check_parse("_курсив_", "курсив", {
        {"italic", 0, 6, ""}
    });

    check_parse("a_b_c", "a_b_c", {});

    check_parse("**не закрыто", "**не закрыто", {});

    check_parse("2 \\* 3", "2 * 3", {});

    check_parse("`чат * список`", "чат * список", {
        {"code", 0, 12, ""}
    });

    check_parse("[документация](https://docs.github.com)", "документация", {
        {"link", 0, 12, "https://docs.github.com"}
    });

    check_parse("**жирный _и курсив_**", "жирный и курсив", {
        {"bold", 0, 15, ""},
        {"italic", 7, 8, ""}
    });

    check_parse("**жирный _и_ курсив**", "жирный и курсив", {
        {"bold", 0, 15, ""},
        {"italic", 7, 1, ""}
    });

    check_parse("🚀 **ок**", "🚀 ок", {
        {"bold", 3, 2, ""}
    });

    check_parse("**🚀 ок**", "🚀 ок", {
        {"bold", 0, 5, ""}
    });

    check_parse("[текст]()", "[текст]()", {});

    check_parse("\\\\", "\\", {});

    check_parse("", "", {});

    check_parse("_ е_", "_ е_", {});
}

void test_utf8_validation() {
    require(msgfmt::is_valid_utf8(""));
    require(msgfmt::is_valid_utf8("Hello, world!"));
    require(msgfmt::is_valid_utf8("Привет, мир! 🚀"));

    require(!msgfmt::is_valid_utf8("\x80"));
    require(!msgfmt::is_valid_utf8("\xFF"));
    require(!msgfmt::is_valid_utf8("\xC0\x80"));
    require(!msgfmt::is_valid_utf8("\xC2"));
    require(!msgfmt::is_valid_utf8("\xE0\x80\x80"));
    require(!msgfmt::is_valid_utf8("\xED\xA0\x80"));
    require(!msgfmt::is_valid_utf8("\xF0\x80\x80\x80"));
    require(!msgfmt::is_valid_utf8("\xF4\x90\x80\x80"));
}

void test_escape() {
    require(msgfmt::escape("") == "");
    require(msgfmt::escape("2 * 3") == "2 \\* 3");
    require(msgfmt::escape("*_`[]()\\") == "\\*\\_\\`\\[\\]\\(\\)\\\\");
    require(msgfmt::escape("Обычный текст 123") == "Обычный текст 123");
}

void test_edge_cases() {
    check_parse("2 ** 3", "2 ** 3", {});
    check_parse("** 3", "** 3", {});
    check_parse("**a **", "**a **", {});
    check_parse("_a _", "_a _", {});
    check_parse("` a`", "` a`", {});
    check_parse("`a `", "`a `", {});

    check_parse("\\a", "\\a", {});
    check_parse("a\\", "a\\", {});

    check_parse("`**жирный** \\_курсив\\_`", "**жирный** \\_курсив\\_", {
        {"code", 0, 21, ""}
    });

    check_parse("**a **b**", "a **b", {
        {"bold", 0, 5, ""}
    });

    check_parse("**_привет_**", "привет", {
        {"bold", 0, 6, ""},
        {"italic", 0, 6, ""}
    });

    check_parse("_**привет**_", "привет", {
        {"italic", 0, 6, ""},
        {"bold", 0, 6, ""}
    });

    check_parse("[текст](https://example.com/a b)", "[текст](https://example.com/a b)", {});
    check_parse("[](https://example.com)", "[](https://example.com)", {});
    check_parse("[a]b](https://example.com)", "[a]b](https://example.com)", {});

    std::string composed_y = "\xD0\xB9 **a**";
    check_parse(composed_y, "\xD0\xB9 a", {
        {"bold", 2, 1, ""}
    });

    std::string decomposed_y = "\xD0\xB8\xCC\x86 **a**";
    check_parse(decomposed_y, "\xD0\xB8\xCC\x86 a", {
        {"bold", 3, 1, ""}
    });
}

void test_json_output() {
    ParsedResult res = msgfmt::parse("Привет, **мир**!");
    std::string json = msgfmt::to_json(res);
    std::string expected = "{\"text\": \"Привет, мир!\", \"ranges\": [{\"style\": \"bold\", \"offset\": 8, \"length\": 3, \"url\": \"\"}]}";
    require(json == expected);
}

int main() {
    test_appendix_a_examples();
    test_utf8_validation();
    test_escape();
    test_edge_cases();
    test_json_output();
    return 0;
}
