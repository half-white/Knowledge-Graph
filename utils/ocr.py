import sys
import io
from contextlib import contextmanager
import os
from paddleocr import PaddleOCR, draw_ocr
from spire.pdf.common import *
from spire.pdf import *
import logging

logging.getLogger("ppocr").setLevel(logging.ERROR)

# 上下文管理器，重定向stdout和stderr
@contextmanager
def suppress_output():
    with open(os.devnull, "w") as fnull:
        old_stdout = sys.stdout
        old_stderr = sys.stderr
        try:
            sys.stdout = fnull
            sys.stderr = fnull
            yield
        finally:
            sys.stdout = old_stdout
            sys.stderr = old_stderr

def GetWord(pdf_path):
    # Paddleocr目前支持的多语言语种可以通过修改lang参数进行切换
    # 例如`ch`, `en`, `fr`, `german`, `korean`, `japan`
    # 识别页码代码
    # pdf_path = '1.pdf'
    pdf_path = 'C:/Users/xieenping/Desktop/实习工作/SSE/utils/'+ pdf_path
    pdf = PdfDocument(pdf_path)
    PAGE_NUM = pdf.Pages.Count # 将识别页码前置作为全局，防止后续打开pdf的参数和前文识别参数不一致 / Set the recognition page number

    with suppress_output():
        ocr = PaddleOCR(use_angle_cls=True, lang="ch", page_num=PAGE_NUM)  # need to run only once to download and load model into memory
        # ocr = PaddleOCR(use_angle_cls=True, lang="ch", page_num=PAGE_NUM,use_gpu=0) # 如果需要使用GPU，请取消此行的注释 并注释上一行 / To Use GPU,uncomment this line and comment the above one.
        result = ocr.ocr(pdf_path, cls=True)

    text = ""
    for idx in range(len(result)):
        res = result[idx]
        if res == None: # 识别到空页就跳过，防止程序报错 / Skip when empty result detected to avoid TypeError:NoneType
            print(f"[DEBUG] Empty page {idx+1} detected, skip it.")
            continue
        for line in res:
            # print(line)
            # print(line[1][0])
            text += line[1][0]
            text += " "
    # print(text)
    return text
    
    # 显示图片对比结果
    # import fitz
    # from PIL import Image
    # import cv2
    # import numpy as np
    # imgs = []
    # with fitz.open(pdf_path) as pdf:
    #     for pg in range(0, PAGE_NUM):
    #         page = pdf[pg]
    #         mat = fitz.Matrix(2, 2)
    #         pm = page.get_pixmap(matrix=mat, alpha=False)
    #         # if width or height > 2000 pixels, don't enlarge the image
    #         if pm.width > 2000 or pm.height > 2000:
    #             pm = page.get_pixmap(matrix=fitz.Matrix(1, 1), alpha=False)
    #         img = Image.frombytes("RGB", [pm.width, pm.height], pm.samples)
    #         img = cv2.cvtColor(np.array(img), cv2.COLOR_RGB2BGR)
    #         imgs.append(img)
    # for idx in range(len(result)):
    #     res = result[idx]
    #     if res == None:
    #         continue
    #     image = imgs[idx]
    #     boxes = [line[0] for line in res]
    #     txts = [line[1][0] for line in res]
    #     scores = [line[1][1] for line in res]
    #     im_show = draw_ocr(image, boxes, txts, scores, font_path='doc/fonts/simfang.ttf')
    #     im_show = Image.fromarray(im_show)
    #     im_show.save('result_page_{}.jpg'.format(idx))
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

if __name__ == "__main__":
    # 从命令行接收参数
    x = sys.argv[1]
    # x = '1.pdf'
    #print(str(x))
    result = GetWord(str(x))  # 调用函数，获取返回值
    print(result)  # 打印返回值，供 Go 捕获